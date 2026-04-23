# Installation

fumi is macOS-only and targets Google Chrome. It is distributed as two Go binaries (`fumi`, `fumi-host`) plus an unpacked Chrome extension. A Homebrew tap and Chrome Web Store listing are planned but not yet available, so the only supported install path today is building from source.

## Requirements

- macOS (Darwin). The CLI refuses to run on other platforms.
- Google Chrome with **Developer mode** enabled. fumi uses `chrome.userScripts`, which Chrome gates behind this flag.
- Go 1.26+ and Node.js 22+ (with pnpm) for building.

## Build

```bash
git clone https://github.com/tkuramot/fumi.git
cd fumi

go build -o ./bin/fumi      ./cmd/fumi
go build -o ./bin/fumi-host ./cmd/fumi-host

cd chrome-extension && pnpm install && pnpm build && cd ..
```

Place `fumi` and `fumi-host` somewhere on your `PATH`. The path to `fumi-host` is baked into the Native Messaging manifest at `fumi setup` time, so if you move the binary later, re-run `fumi setup --force`.

### Build-time variables

The two binaries embed three values at build time via `-ldflags`. The dev defaults work for a from-source install, but production builds should override them:

| Variable | Default | Purpose |
|---|---|---|
| `main.hostBinaryPath` | `/opt/homebrew/bin/fumi-host` | Path written into the Native Messaging manifest |
| `main.webStoreExtensionID` | 32 × `a` | Chrome Web Store extension ID, pinned in `allowed_origins` |
| `main.unpackedExtensionID` | 32 × `b` | Unpacked extension ID, pinned in `allowed_origins` |

For a local unpacked install, set `main.unpackedExtensionID` to match your unpacked extension's ID (see below).

## Install

### 1. Initialize the store and manifest

```bash
fumi setup
```

This does, in order:

1. Creates the store at `~/.config/fumi/` (or `$FUMI_STORE` if set). Subdirectories `actions/` and `scripts/` are created with mode `0700`.
2. Writes a template `config.toml` at the store root (mode `0600`).
3. Writes the Native Messaging manifest to `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/com.tkrmt.fumi.json`.

Useful flags:

- `--force` — overwrite an existing manifest (safe; does not touch the store).
- `--store-root PATH` — use a non-default store location.
- `--manifest-dir PATH` — write the manifest to a custom directory (e.g. for Chrome Canary).

`fumi setup` does **not** create sample actions or scripts.

### 2. Load the Chrome extension (unpacked)

1. Open `chrome://extensions` and enable **Developer mode**.
2. Click **Load unpacked** and select `chrome-extension/dist`.
3. Copy the **Extension ID** shown on the card.

### 3. Pin the extension ID

The manifest's `allowed_origins` must contain your exact unpacked ID or Chrome will refuse to connect to the host. If the ID you copied does not match what was baked into your build, rebuild with:

```bash
go build \
  -ldflags "-X main.unpackedExtensionID=<your-extension-id>" \
  -o ./bin/fumi ./cmd/fumi
fumi setup --force
```

To keep the ID stable across reinstalls, set the `"key"` field in `chrome-extension/public/manifest.json` to a base64 public key you control before building; see Chrome's [extension key documentation](https://developer.chrome.com/docs/extensions/reference/manifest/key).

### 4. Verify

```bash
fumi doctor
```

All rows should be `[OK]`. See [troubleshooting.md](./troubleshooting.md) if any are `[NG]`.

## Updating

```bash
git pull
go build -o ./bin/fumi ./cmd/fumi
go build -o ./bin/fumi-host ./cmd/fumi-host
(cd chrome-extension && pnpm build)
fumi setup --force          # only needed if hostBinaryPath changed
```

Then click **Reload** on the extension card in `chrome://extensions`.

## Uninstall

```bash
fumi uninstall
```

This removes the Native Messaging manifest only. Your store (`~/.config/fumi/`) is left untouched so your actions and scripts survive. Delete it manually if you want a clean slate:

```bash
rm -rf ~/.config/fumi
```

Then remove the extension from `chrome://extensions`.

## Known limitations

- Only the default Chrome install is detected. Chrome Canary, Chromium, Chrome Beta, and Chrome Dev each use their own NativeMessagingHosts directory — use `--manifest-dir` to target them.
- Only one `unpackedExtensionID` is pinned per build. If you load the same extension into multiple Chrome profiles with different IDs, only one will work at a time.
- Firefox, Edge, Safari, Linux, and Windows are not supported.
