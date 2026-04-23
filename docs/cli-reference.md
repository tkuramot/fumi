# CLI reference

All commands are subcommands of `fumi`. Invoke with `--help` on any subcommand for flag detail.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | CLI usage error (bad flags, missing arguments) |
| 2 | Domain error (manifest missing, validation failed, `doctor` found problems, script failed when `--propagate-exit` is set) |
| 3 | Internal bug (unexpected panic) |

## `fumi setup`

Initializes the store and installs the Native Messaging manifest.

```
fumi setup [--browser chrome] [--force] [--store-root PATH] [--manifest-dir PATH]
```

| Flag | Default | Description |
|---|---|---|
| `--browser` | `chrome` | Target browser. Only `chrome` is supported. |
| `--force` | off | Overwrite an existing manifest. Never touches the store. |
| `--store-root` | `$FUMI_STORE` or `~/.config/fumi` | Store root to initialize. |
| `--manifest-dir` | Chrome's default `NativeMessagingHosts` | Directory to write the manifest into. |

Idempotent. Running twice without `--force` leaves the manifest in place; the store is created only if missing.

## `fumi doctor`

Reports the state of the installation. Prints one line per check.

```
fumi doctor [--browser chrome] [--manifest-dir PATH] [--store-root PATH]
```

Checks performed:

1. Native Messaging manifest exists.
2. Manifest's `allowed_origins` matches the extension IDs embedded at build time.
3. `fumi-host` binary exists and is executable.
4. `config.toml` parses.
5. Store root path resolves.
6. Store root is mode `0700`.
7. `actions/` and `scripts/` exist; counts files in each.
8. Config is semantically valid.

Exits with code 2 if any row is `[NG]`, otherwise 0. Rows may be `[OK]`, `[WARN]`, or `[NG]`.

## `fumi actions list`

Prints a tab-separated table of actions found in `~/.config/fumi/actions/`: ID, filename, match patterns.

```
fumi actions list
```

Frontmatter errors are reported per-file. Invalid actions are listed with their error and do not abort the command.

## `fumi scripts list`

Recursively lists files under `~/.config/fumi/scripts/`, with their type (`file`, `symlink`, `dir`, ...) and whether they are executable.

```
fumi scripts list
```

Use this to confirm a new script is visible and has the `+x` bit set.

## `fumi scripts run`

Invokes a script with the same validation and spawning rules the extension uses. Useful for local debugging.

```
fumi scripts run <name> [--payload JSON] [--timeout MS] [--json] [--propagate-exit]
```

| Flag | Default | Description |
|---|---|---|
| `--payload` | `null` | JSON value written to the script's stdin. |
| `--timeout` | `30000` | Milliseconds before SIGTERM → SIGKILL. |
| `--json` | off | Print the full `RunResult` (`exitCode`, `stdout`, `stderr`, `durationMs`) as pretty JSON. |
| `--propagate-exit` | off | Exit with the script's own exit code instead of 0. Combine with `--json` to both display and propagate. |

Without `--json`, stdout is printed to stdout and stderr to stderr, matching the terminal UX.

## `fumi uninstall`

Removes the Native Messaging manifest. **Does not touch the store.**

```
fumi uninstall [--browser chrome] [--all-browsers] [--manifest-dir PATH]
```

| Flag | Default | Description |
|---|---|---|
| `--browser` | `chrome` | Target browser. |
| `--all-browsers` | off | Remove the manifest for every supported browser. Currently equivalent to the default since only Chrome is supported. |
| `--manifest-dir` | Chrome's default | Override the directory to remove from. |

If the manifest is already missing, the command logs `[skip]` and exits 0.

## Environment variables

| Variable | Read by | Effect |
|---|---|---|
| `FUMI_STORE` | `fumi`, `fumi-host` | Overrides the default store root (`~/.config/fumi`). |

`fumi-host` accepts no flags; Chrome invokes it with internal arguments via the Native Messaging protocol.
