# Security model

fumi lets web pages trigger local executables. That is an intentionally dangerous capability, so the design is conservative: the bridge between the browser and your shell exposes the smallest possible surface, and almost every restriction is enforced on the host side where an attacker cannot reach.

## Threat model

### What fumi protects against

- **A compromised web page** running an action. The page cannot forge calls with arbitrary paths, read files outside `scripts/`, or pass shell-interpreted arguments.
- **A malicious or buggy action** trying to escape. It still cannot resolve paths outside `scripts/`, follow symlinks out, or execute non-script files.
- **An unrelated extension or process** trying to talk to `fumi-host`. The manifest's `allowed_origins` pins exactly the extension IDs baked into the build; Chrome refuses to broker connections from anyone else.

### What fumi does **not** protect against

- **The host machine already being compromised.** If an attacker can write to `~/.config/fumi/scripts/`, they can run anything you can run. fumi assumes the filesystem is trustworthy.
- **Scripts you write yourself.** fumi will happily execute `rm -rf ~` if you put it in a script. Treat every script as code you own and audit.
- **Payload-level attacks.** If your script parses the payload carelessly (shell interpolation, SQL concatenation, arbitrary code execution), fumi cannot help. Treat payloads as untrusted input.
- **A compromised Chrome profile or extension signing key.** If the attacker controls the pinned extension ID, they can call the host. Protect the extension's `"key"` accordingly.

## Host surface

`fumi-host` speaks JSON-RPC 2.0 over Chrome's Native Messaging stdio transport. The entire API is two methods:

- `actions/list` — returns parsed frontmatter from `actions/*.js`. Read-only.
- `scripts/run` — executes a file in `scripts/` with a JSON payload on stdin.

There is no file read, no file write, no directory list, no environment introspection, no "eval this code", and no way to pass a path or command outside of a `scripts/` entry. Adding one would require a new method, a host rebuild, and a `fumi setup --force`.

## Script resolution rules

Every `scripts/run` call runs the candidate path through these checks, in order. Any failure aborts before spawn.

| Rejected when | Error code |
|---|---|
| Path is absolute or contains `..` | `SCRIPT_INVALID_PATH` |
| Resolved real path is outside `scripts/` | `SCRIPT_INVALID_PATH` |
| File does not exist | `SCRIPT_NOT_FOUND` |
| File is a symlink (before or after resolution) | `SCRIPT_NOT_REGULAR_FILE` |
| File is not a regular file (directory, device, FIFO, socket) | `SCRIPT_NOT_REGULAR_FILE` |
| Owner-executable bit is unset | `SCRIPT_NOT_EXECUTABLE` |

Symlinks are rejected rather than followed. A compromised action cannot plant a symlink to `/bin/sh` inside `scripts/` to escape.

## Spawn rules

- **No shell.** Scripts are `exec`'d directly; there is no `sh -c`, so shell metacharacters in payloads are inert.
- **No argv injection.** The host never passes user-controlled arguments. `argv[0]` is the script path and nothing else.
- **Stdin-only payload.** The JSON payload is written to the child's stdin. A script that never reads stdin cannot be influenced by the payload at all.
- **Environment scrubbed.** All `FUMI_*` variables from the parent environment are dropped; only `FUMI_STORE` is re-set. Callers cannot smuggle state via environment variables.
- **Working directory pinned.** Cwd is the directory containing the script, not wherever the extension happened to start.

## Resource caps

These exist to keep a single bad script from hanging or flooding the host:

| Cap | Default | Error on overflow |
|---|---|---|
| Wall-clock runtime | 30 s (configurable) | `EXEC_TIMEOUT` (SIGTERM, then SIGKILL after 500 ms) |
| Stdout captured | 768 KiB | `EXEC_OUTPUT_TOO_LARGE` |
| Stderr captured | 128 KiB | `EXEC_OUTPUT_TOO_LARGE` |
| Native Messaging message | 1 MiB | `PROTO_PARSE_ERROR` / `INTERNAL` |

These are hard limits enforced in the host. A runaway script is killed; a flood of output is truncated and the call rejects.

## Origin pinning

The Native Messaging manifest's `allowed_origins` list contains exactly two IDs, both compiled into the `fumi` binary at build time:

- The Chrome Web Store extension ID.
- The unpacked / development extension ID.

Any other extension that tries to open a port to `com.tkrmt.fumi` is rejected by Chrome before `fumi-host` is even spawned. Changing the pinned IDs requires rebuilding `fumi` and re-running `fumi setup --force`.

## What stays on disk

- The store at `~/.config/fumi/` is created with mode `0700`; only your user can read it.
- `config.toml` is `0600`.
- `fumi uninstall` removes the manifest but leaves the store, so removing fumi does not delete your scripts. Delete `~/.config/fumi/` manually if that is what you want.

## Error codes

Error codes are returned in JSON-RPC `error.code` and are stable across versions. Protocol-level codes (`PROTO_*`, `INTERNAL`) match JSON-RPC 2.0 conventions; domain codes are in the `-33xxx` range.

| Code | Numeric | When |
|---|---|---|
| `PROTO_PARSE_ERROR` | -32700 | JSON parse failed |
| `PROTO_INVALID_REQUEST` | -32600 | Missing/invalid required fields |
| `PROTO_METHOD_NOT_FOUND` | -32601 | Unknown method |
| `PROTO_INVALID_PARAMS` | -32602 | Param validation failed |
| `INTERNAL` | -32603 | Host-side bug |
| `STORE_NOT_FOUND` | -33001 | Store root missing |
| `STORE_CONFIG_INVALID` | -33002 | `config.toml` parse error |
| `STORE_ACTIONS_TOO_LARGE` | -33010 | `actions/list` response exceeds 1 MiB |
| `STORE_FRONTMATTER_INVALID` | -33011 | Action frontmatter rejected |
| `SCRIPT_INVALID_PATH` | -33020 | Path traversal / outside `scripts/` |
| `SCRIPT_NOT_FOUND` | -33021 | Script missing |
| `SCRIPT_NOT_REGULAR_FILE` | -33022 | Symlink or non-regular file |
| `SCRIPT_NOT_EXECUTABLE` | -33023 | Owner-executable bit unset |
| `EXEC_TIMEOUT` | -33030 | Timeout exceeded |
| `EXEC_OUTPUT_TOO_LARGE` | -33031 | Stdout or stderr overflow |
| `EXEC_SPAWN_FAILED` | -33032 | `exec` itself failed |
