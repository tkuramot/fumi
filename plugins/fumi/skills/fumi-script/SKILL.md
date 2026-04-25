---
name: fumi-script
description: Create a new fumi host script in ~/.config/fumi/scripts/ that fumi actions can invoke via fumi.run
argument-hint: <what the script should do, e.g. "append a JSON line to ~/notes/prs.ndjson">
---

Create a new fumi host script based on the user's request.

## Request

$ARGUMENTS

## Steps

1. **Pick the language.** Default to `bash` for simple I/O, `python3` for JSON munging, `node` only if needed. Ask if unclear.
2. **Pick a path** under `~/.config/fumi/scripts/`. Subdirectories are fine (`notes/append.py`). Confirm it doesn't already exist.
3. **Write the file** with a shebang on line 1 and `set -euo pipefail` (bash) or equivalent. Read the entire payload from stdin to EOF, then process it.
4. **`chmod +x`** the file — without the owner-executable bit, fumi rejects it with `SCRIPT_NOT_EXECUTABLE`.
5. **Test from the CLI** before wiring it into an action:

   ```bash
   fumi scripts run <relpath> --payload '{"...":"..."}'
   ```

6. **Tell the user how to call it from an action**: `await fumi.run('<relpath>', payload)`. Offer the `fumi-action` skill if they need one.

## Invocation contract (preserve when writing scripts)

- **Argv**: only `argv[0]`. No user-controlled arguments are ever passed — payload is stdin only.
- **Stdin**: raw JSON encoding of `payload`, single write, no trailing newline. If payload is absent/null, the literal bytes `null` are written. Stdin closes after the write.
- **Cwd**: the directory containing the script. Relative paths resolve next to the script.
- **No shell**: `exec`'d directly, no `sh -c`, no expansion. Do not try to interpolate payload into a command line — parse it as JSON.
- **Env**: inherited, but all `FUMI_*` are stripped, then `FUMI_STORE` is re-set to the store root. The variable name `store` is reserved.

## Hard rules

- Shebang required (`#!/usr/bin/env bash`, `#!/usr/bin/env python3`, etc.).
- Owner-executable bit required (`chmod +x`).
- No symlinks — fumi rejects them. To share code, copy or hard-link.
- Stdout capped at 768 KiB, stderr at 128 KiB. Overflow rejects the call with `EXEC_OUTPUT_TOO_LARGE`. For bulk data, write to a file and return the path.
- Default timeout 30 s; on timeout the script gets SIGTERM, then SIGKILL after 500 ms. For background work, spawn a detached process and return immediately.
- Treat the payload as untrusted — even if the action is yours, the page DOM can be hostile. Parse JSON, never `eval` or interpolate into a shell command.
- Fail loudly: non-zero exit + descriptive stderr beats silent success with an error in stdout.

## Bash skeleton

```bash
#!/usr/bin/env bash
set -euo pipefail

payload="$(cat)"
field=$(jq -r '.field' <<<"$payload")

# … do the thing …

printf '%s\n' "ok"
```

## Python skeleton

```python
#!/usr/bin/env python3
import json, sys

payload = json.load(sys.stdin)
# … do the thing …
print("ok")
```
