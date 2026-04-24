# fumi

> Bridge the browser and your host machine. Write userscripts that call local executables.

**fumi** is a Chrome extension + native messaging host that lets you run JavaScript in any web page and invoke scripts on your machine from it — all managed as plain files in your editor, under version control.

Think Tampermonkey, but your userscripts can shell out to anything on your box.

```js
// ==Fumi Action==
// @match https://github.com/*
// ==/Fumi Action==

document.addEventListener('keydown', async (e) => {
  if (e.ctrlKey && e.shiftKey && e.key === 'S') {
    const { stdout } = await fumi.run('save-note.sh', {
      title: document.title,
      url: location.href,
      selection: String(window.getSelection()),
    });
    console.log('saved:', stdout);
  }
});
```

## Why fumi

- **Files, not a UI.** Actions and scripts live in `~/.config/fumi/` — edit with your editor, track with git, back up like any other dotfile.
- **Any language for host scripts.** `fumi.run("foo.py", payload)` — bash, Python, Go binary, whatever is executable.
- **Small, auditable surface.** The host exposes exactly two operations: list actions, run a script. No remote file I/O, no shell interpolation, no writes from the browser.
- **Tampermonkey-style frontmatter.** `// @match`, `// @exclude` — familiar and declarative.
- **Zero state in the browser.** The extension is a thin runner; the filesystem is the source of truth.

## How it works

```
[Web Page] ⇄ [User Script] ⇄ [Extension SW] ⇄ [fumi-host] ⇄ [your script]
                                                   ⇣
                                       ~/.config/fumi/{actions,scripts}/
```

- `chrome.userScripts` runs your action JS on matched pages.
- `fumi.run(name, payload)` sends a Native Messaging request to `fumi-host`.
- `fumi-host` spawns `~/.config/fumi/scripts/<name>` directly (no shell), pipes `payload` as JSON on stdin, and returns `{ exitCode, stdout, stderr, durationMs }`.

## Requirements

- macOS (Linux / Windows not supported)
- Google Chrome with **Developer mode** enabled (required for `chrome.userScripts`)
- Go 1.26+ and Node.js 22+ (for building from source)

### Distribution

| Channel | Status |
|---|---|
| Homebrew tap (`brew install --cask tkuramot/tap/fumi`) | available |
| GitHub Releases (binaries + extension zip) | available |
| Chrome Web Store listing | *TBD* |

The extension still has to be loaded unpacked until the Chrome Web Store listing lands, but you no longer need to build the Go binaries or the extension yourself — download the pre-built zip from the latest [GitHub release](https://github.com/tkuramot/fumi/releases).

## Quick start

### 1. Install the binaries

```bash
brew install --cask tkuramot/tap/fumi
```

This installs both `fumi` and `fumi-host` to `/opt/homebrew/bin` (the path the Native Messaging manifest expects by default). See [docs/installation.md](./docs/installation.md) for other install paths, including building from source.

### 2. Set up the native host and store

```bash
fumi setup
```

This places the Native Messaging manifest, creates `~/.config/fumi/{actions,scripts}/` (mode 0700), and drops in a couple of samples.

### 3. Load the Chrome extension (unpacked)

1. Download `fumi-extension_<version>.zip` from the latest [GitHub release](https://github.com/tkuramot/fumi/releases) and unzip it (or use `chrome-extension/dist` if you built from source).
2. Visit `chrome://extensions` and enable **Developer mode**.
3. Click **Load unpacked** and select the unzipped directory.

### 4. Verify

```bash
fumi doctor
```

Should report a green manifest, matching Extension ID, and a writable store.

### 5. Write your first action

```bash
$EDITOR ~/.config/fumi/actions/hello.js
```

```js
// ==Fumi Action==
// @match https://example.com/*
// ==/Fumi Action==

const { stdout } = await fumi.run('hello.sh', { url: location.href });
alert(stdout);
```

```bash
cat > ~/.config/fumi/scripts/hello.sh <<'EOF'
#!/usr/bin/env bash
read -r payload
echo "hello from $payload"
EOF
chmod +x ~/.config/fumi/scripts/hello.sh
```

Open the extension popup → **Reload actions**, then visit `https://example.com`.

## CLI overview

| Command | Purpose |
|---|---|
| `fumi setup` | Install native messaging manifest and initialize the store |
| `fumi doctor` | Diagnose install / permissions / Extension ID mismatches |
| `fumi actions list` | List actions in the store |
| `fumi scripts list` | List scripts in the store |
| `fumi scripts run <name> [--payload '<json>']` | Invoke a script from the shell for debugging |
| `fumi uninstall` | Remove the native messaging manifest (the store is preserved) |

## Security model

fumi is designed so a compromised page or extension **cannot reach beyond your own scripts**:

- The host has **no** write, read, list, or arbitrary-path API — only `actions/list` and `scripts/run`.
- Script paths are resolved with `realpath` + `lstat`, rejecting anything outside `scripts/`, any symlink, and anything that isn't a regular file.
- Scripts are spawned directly (no shell); payloads arrive on **stdin only**, never as argv.
- `allowed_origins` is pinned to fumi's Extension IDs.

For the full threat model, see [docs/security.md](./docs/security.md).

## Documentation

- [Installation](./docs/installation.md) — build, setup, extension loading, updates, uninstall
- [Authoring actions](./docs/authoring-actions.md) — frontmatter, match patterns, the `fumi.run` API
- [Authoring scripts](./docs/authoring-scripts.md) — stdin/stdout contract, env vars, limits
- [CLI reference](./docs/cli-reference.md) — every subcommand, flags, exit codes
- [Security model](./docs/security.md) — threat model, resolution rules, error codes
- [Troubleshooting](./docs/troubleshooting.md) — common errors and fixes

## Status

Early development. Expect breaking changes.

## License

[MIT](./LICENSE)
