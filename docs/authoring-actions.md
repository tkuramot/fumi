# Authoring actions

An **action** is a JavaScript file in `~/.config/fumi/actions/` that fumi injects into matching web pages. Actions can call host scripts through the `fumi.run` API.

## File layout

```
~/.config/fumi/
  actions/
    my-action.js
    gh-pr-notes.js
  scripts/
    ...
```

Every `.js` file directly under `actions/` is an action. Subdirectories are ignored. The extension re-reads this directory when you click **Reload actions** in the popup — changes are not picked up live.

## Frontmatter

Every action must begin with a frontmatter block. Without one, the file is rejected when `actions/list` is called.

```js
// ==Fumi Action==
// @id github-pr-notes
// @match https://github.com/*/pull/*
// @exclude https://github.com/*/pull/*/files
// ==/Fumi Action==

// …action code here…
```

### Delimiters

- Start: `// ==Fumi Action==`
- End: `// ==/Fumi Action==`

Leading whitespace is allowed; the tokens themselves are case-sensitive.

### Supported directives

| Directive | Repeats | Required | Meaning |
|---|---|---|---|
| `@id` | no | no | Stable action ID. If omitted, derived from the filename. |
| `@match` | yes | yes (≥1) | A [Chrome match pattern](https://developer.chrome.com/docs/extensions/develop/concepts/match-patterns). Pages matching any `@match` are eligible. |
| `@exclude` | yes | no | A match pattern that excludes pages even if they match. |

Directive names must be lowercase. Any unknown directive is a hard error — the file is rejected, not ignored. Duplicate `@id` values (including ones derived from filenames) collide and both files fail validation.

### ID derivation

If `@id` is omitted, the ID is derived from the filename by:

1. Stripping the `.js` extension.
2. Replacing runs of non-alphanumeric characters with `-`.
3. Trimming leading/trailing `-`.
4. Lowercasing.

So `My Action 1.js` → `my-action-1`.

## Match patterns

Patterns are passed verbatim to `chrome.userScripts.register`, so Chrome's rules apply. In short:

- Scheme: `http`, `https`, `file`, or `*` (http+https).
- Host: a literal hostname, `*`, or `*.example.com`.
- Path: any string; `*` matches any sequence.
- `<all_urls>` matches every URL supported by Chrome.

Invalid patterns cause the extension to fail registration for that action; you'll see the error in the service worker console.

## Execution environment

- **World**: `USER_SCRIPT` — isolated from the page's JavaScript context. You cannot touch page globals directly; use `window.wrappedJSObject`-style techniques or `MAIN`-world scripts elsewhere if you need to. This isolation is why a hostile page cannot tamper with `fumi.run`.
- **Timing**: `document_idle` — after the DOM is ready and most resources have loaded.
- **Injection order**: a small prelude runs first to define the `fumi` global, then your action code.
- **Storage**: none provided. Use `chrome.storage` (not available in USER_SCRIPT world) from a companion extension if needed, or persist state from a host script.

## The `fumi` API

Only one method is available today.

### `fumi.run(scriptPath, payload, opts?)`

Runs a host script in `~/.config/fumi/scripts/` and resolves with its result.

```ts
fumi.run(
  scriptPath: string,
  payload: unknown,
  opts?: { timeoutMs?: number }
): Promise<RunResult>

type RunResult = {
  exitCode: number;
  stdout: string;
  stderr: string;
  durationMs: number;
};
```

- **`scriptPath`** — path relative to `scripts/`, without a leading `/`. `..` segments are rejected.
- **`payload`** — any JSON-serializable value. It is written to the script's stdin as raw JSON bytes (no trailing newline). If omitted or `null`, the literal string `null` is written.
- **`opts.timeoutMs`** — overrides the default (30s, or `default_timeout_ms` in `config.toml`). On timeout the host sends SIGTERM, waits 500ms, then SIGKILL, and the promise rejects with an `EXEC_TIMEOUT` error.

The promise **resolves** as long as the script spawned and exited, even if `exitCode !== 0`. Inspect `exitCode` yourself.

The promise **rejects** when the request never reached a script: invalid path, missing script, wrong file mode, output-too-large, spawn failure, timeout, or a protocol error. The rejection is an `Error` whose `.message` is the host's error message; the error code (see [security.md](./security.md)) is not currently surfaced programmatically.

### Concurrency

Each call round-trips through Chrome's Native Messaging channel. Calls from the same action are serialized by the extension's message queue; calls from different tabs or actions run in parallel host processes — each `scripts/run` spawns its own `fumi-host` process.

### Example

```js
// ==Fumi Action==
// @match https://github.com/*/pull/*
// ==/Fumi Action==

document.addEventListener('keydown', async (e) => {
  if (!(e.ctrlKey && e.shiftKey && e.key === 'S')) return;

  try {
    const { exitCode, stdout, stderr } = await fumi.run('save-pr.sh', {
      url: location.href,
      title: document.title,
    }, { timeoutMs: 5000 });

    if (exitCode === 0) console.log('saved:', stdout.trim());
    else console.error('save-pr failed:', stderr);
  } catch (err) {
    console.error('fumi.run rejected:', err);
  }
});
```

## What actions cannot do

- Modify or read other actions' files.
- Read or write arbitrary host filesystem paths. Everything goes through `fumi.run`, which only executes things in `scripts/`.
- Stream output. `stdout`/`stderr` arrive only after the script exits.
- Receive output larger than 768 KiB stdout or 128 KiB stderr — excess is truncated and the call rejects with `EXEC_OUTPUT_TOO_LARGE`.
- Persist the `fumi` object or cache results across reloads.

## Reloading after edits

1. Save your `.js` file.
2. Open the fumi extension popup and click **Reload actions**.
3. Reload the target page.

Editing an action while a page is already loaded does not update that page — navigate away and back, or reload.
