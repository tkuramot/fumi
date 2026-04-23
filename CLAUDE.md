# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

fumi is a Chrome extension + Go native-messaging host that lets userscripts running in web pages invoke executables on the host machine. The browser never sees the filesystem directly — it speaks JSON-RPC 2.0 over Chrome Native Messaging to the `fumi-host` binary, which is the only component with filesystem access.

The store is `~/.config/fumi/{actions,scripts}/` (mode 0700). `actions/*.js` are Tampermonkey-style userscripts (with `// ==Fumi Action==` frontmatter declaring `@match` / `@exclude` / `@id`); `scripts/*` are executables they invoke via `fumi.run(name, payload)`.

## Build and test

Dev shell (optional, via `nix develop` or direnv): Go 1.26+, Node 22+, pnpm, TypeScript.

```bash
# Go binaries
go build -o ./bin/fumi       ./cmd/fumi
go build -o ./bin/fumi-host  ./cmd/fumi-host

# Chrome extension (produces chrome-extension/dist/)
cd chrome-extension && pnpm install && pnpm build
```

Extension scripts (run inside `chrome-extension/`):
- `pnpm typecheck` — `tsc --noEmit`
- `pnpm test` — compiles then runs `node --test 'dist/**/*.test.js'`
- `pnpm lint` / `pnpm format` / `pnpm check` — Biome (double-quote style)

Run one extension test: `pnpm exec tsc -p tsconfig.json && node --test dist/background/actions.test.js`.

There are currently no Go tests in the repo; `go test ./...` is a no-op but fine to run.

Release builds override identity constants via ldflags (see `cmd/fumi/constants.go`):
`-X main.webStoreExtensionID=... -X main.unpackedExtensionID=... -X main.hostBinaryPath=...`.

## Architecture

### Process model

```
web page  ⇄  userscript (chrome.userScripts)  ⇄  background SW  ⇄  fumi-host  ⇄  child script
```

- `fumi-host` is **short-lived**: Chrome spawns one process per request; it reads a single Native Messaging frame from stdin, dispatches, writes one reply, exits. See `cmd/fumi-host/main.go` and `dispatch.go`. Do not add long-running state to the host.
- The extension service worker owns all `chrome.*` APIs and routes userscript → host messages. Entry at `chrome-extension/src/background/index.ts`; Chrome-facing adapters under `background/chrome/`.
- The userscript prelude (`src/userscript/prelude.ts`) injects the `fumi.run` API into matched pages.

### Wire protocol

JSON-RPC 2.0 over Native Messaging (4-byte little-endian length prefix, 1 MiB cap). Only two methods exist and the surface is deliberately minimal:

- `actions/list` → `{ actions: Action[] }` (id, path, matches, excludes, code)
- `scripts/run` → `{ exitCode, stdout, stderr, durationMs }`

Types in `internal/protocol/types.go`; framing in `internal/protocol/codec.go`; error taxonomy (`PROTO_*`, `STORE_*`, `EXEC_*`, `INTERNAL`) in `internal/protocol/errors.go`. Batch requests and notifications without `id` are handled explicitly in `dispatch.go` — preserve that behavior when changing dispatch.

### Store resolution

`internal/config` loads `~/.config/fumi/config.toml` (missing file = defaults, not error). `internal/store/paths.go#Resolve` picks the store root with priority **`$FUMI_STORE` > `config.store_root` > OS default**, expands `~`, and returns `{Root, Actions, Scripts}`. Any code that needs store paths should go through `store.Resolve(cfg)` rather than reconstructing paths.

### Script execution (security-critical)

`internal/runner/runner.go` is the only place that spawns child processes. Invariants to preserve:

- Scripts are spawned **directly via `exec.Command`** — never through a shell, never with payload as argv. Payload is written to the child's **stdin only**.
- Script paths are pre-resolved by `internal/store/scripts.go` with `realpath` + `lstat`, rejecting symlinks, non-regular files, and anything outside `scripts/`. The runner accepts only a `*store.ResolvedScript`.
- Env is scrubbed of inherited `FUMI_*` before re-adding `FUMI_STORE` and `FUMI_*` context keys (camelCase → `SCREAMING_SNAKE`, validated against `envKeyRe`). `FUMI_STORE` is reserved.
- stdout capped at 768 KiB, stderr at 128 KiB (`cappedBuffer`); overflow returns `EXEC_OUTPUT_TOO_LARGE`. Timeout uses `context.WithTimeout` + SIGTERM, 500ms grace, then SIGKILL; timeout reports `EXEC_TIMEOUT`.

### Frontmatter

`internal/store/frontmatter.go` parses the `// ==Fumi Action== … // ==/Fumi Action==` block. Only `@id`, `@match`, `@exclude` are recognized — unknown keys are a parse error. Non-comment code before the start marker means "no frontmatter" (not an error); code inside the block is a parse error.

### CLI exit code contract

`cmd/fumi/main.go` defines the convention and all subcommands must honor it:

- `0` success · `1` usage error · `2` domain error (doctor NG, validation failed, missing manifest) · `3` internal bug

Use `cli.Exit(msg, exitDomain)` for expected failures; return plain errors only for unexpected ones (they map to exit 3).

## Conventions

- When touching the host/runner, keep it boring: no long-lived goroutines beyond the existing timeout watcher, no filesystem APIs outside `internal/store`, no new JSON-RPC methods without also updating `docs/security.md` (the surface is part of the threat model).
- TypeScript in the extension uses double quotes (Biome-enforced). Tests are plain `node:test` on the compiled `dist/` output — source tests must survive a straight `tsc` build.
- macOS-only. Don't add Linux/Windows paths unless asked.
