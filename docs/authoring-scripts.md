# Authoring host scripts

A **script** is any executable file under `~/.config/fumi/scripts/` that an action can invoke via `fumi.run`. Scripts can be written in any language — bash, Python, Node, Ruby, a compiled Go binary, anything your shell can run.

## Location and layout

```
~/.config/fumi/scripts/
  hello.sh
  save-pr.sh
  notes/
    append.py
```

Subdirectories are allowed. When calling, use the path relative to `scripts/`:

```js
await fumi.run('notes/append.py', { ... });
```

Absolute paths and parent-traversal (`..`) segments are rejected before the script runs.

## Validation

Before spawning, fumi checks every candidate path. Any of these cause the call to reject without the script running:

| Condition | Error code |
|---|---|
| Path contains `..` or starts with `/` | `SCRIPT_INVALID_PATH` |
| Resolved path falls outside `scripts/` | `SCRIPT_INVALID_PATH` |
| File does not exist | `SCRIPT_NOT_FOUND` |
| Target is a symlink | `SCRIPT_NOT_REGULAR_FILE` |
| Target is a directory, socket, device, FIFO, etc. | `SCRIPT_NOT_REGULAR_FILE` |
| Target lacks the executable bit for the owner | `SCRIPT_NOT_EXECUTABLE` |

Symlinks are rejected on purpose — they would let a compromised action escape the `scripts/` directory. If you want to share code between scripts, copy or hard-link the file, or have scripts call each other explicitly.

## Invocation contract

When a script runs, fumi guarantees:

- **Argv**: only `argv[0]` (the script path). No user-controlled arguments are ever passed.
- **Stdin**: the raw JSON encoding of `payload`, written in a single `write`, no trailing newline. If `payload` is absent or `null`, the literal bytes `null` are written. Stdin is closed when the payload has been written.
- **Cwd**: the directory containing the script. Relative paths in your script resolve next to the script itself.
- **Shell**: none. The script is `exec`'d directly; there is no `sh -c`, no expansion, no word-splitting.

### Environment variables

The inherited environment is passed through, with two adjustments:

1. Any variable whose name starts with `FUMI_` is stripped, to prevent the caller from spoofing fumi's own channels.
2. `FUMI_STORE` is always set to the absolute path of the store root.

The variable name `store` is reserved and rejected.

### Reading the payload

Typical pattern for a shell script:

```bash
#!/usr/bin/env bash
set -euo pipefail
payload="$(cat)"            # JSON
url=$(jq -r .url <<<"$payload")
echo "received: $url"
```

Python:

```python
#!/usr/bin/env python3
import json, sys
payload = json.load(sys.stdin)
print("received:", payload["url"])
```

Always read stdin to EOF before writing output; fumi closes stdin once the payload is written, so `read` / `json.load` will return.

## Output contract

- **Stdout** is captured up to **768 KiB**. Anything beyond that is discarded and the call rejects with `EXEC_OUTPUT_TOO_LARGE` (even if the script then exits 0).
- **Stderr** is captured up to **128 KiB**, same rule.
- Both are returned as UTF-8 strings. Non-UTF-8 bytes are replaced by the Unicode replacement character when the host serializes the response as JSON.
- **Exit code** is returned verbatim as `result.exitCode`.
- **Duration** is wall-clock milliseconds from spawn to exit, returned as `result.durationMs`.

Stdout and stderr are buffered — they are not streamed to the action. The action sees them only after the script exits.

## Timeouts

The default timeout is 30 seconds. Callers can override it per call (`fumi.run(name, payload, { timeoutMs })`), or you can change the default in `config.toml`:

```toml
default_timeout_ms = 10000
```

On timeout:

1. `SIGTERM` is sent to the script.
2. 500 ms later, if it is still alive, `SIGKILL` is sent.
3. The call rejects with `EXEC_TIMEOUT`; whatever output was captured up to that point is discarded.

Long-running background work does not fit fumi. If you need it, have the script spawn a detached process and return immediately.

## Best practices

- **Shebang required.** Scripts without a shebang will only run if the kernel knows how to exec them (usually not).
- **`chmod +x` every new script.** The CLI will tell you if a script is not executable; actions will see `SCRIPT_NOT_EXECUTABLE`.
- **Treat payloads as untrusted.** Even if you wrote the action yourself, anything that builds a payload from page DOM can be influenced by a hostile page. Parse JSON; do not `eval` or interpolate into shell.
- **Fail loudly.** Non-zero exit with a descriptive stderr is easier for the action to debug than silent success with an error in stdout.
- **Keep output small.** JSON blobs under a few hundred KB are fine. For bulk data, write to a file and return the path.
- **Prefer pure functions.** A script that only reads stdin and writes stdout is easy to reason about and easy to test with `fumi scripts run`.

## Testing from the CLI

You can invoke scripts without the extension:

```bash
fumi scripts run hello.sh --payload '{"url":"https://example.com"}'
fumi scripts run hello.sh --payload '{"x":1}' --json
fumi scripts run hello.sh --propagate-exit   # exit with the script's own code
```

This exercises the exact same validation, spawning, and capture path as the extension, so a script that works under `fumi scripts run` will work from an action (assuming the payload shape matches).
