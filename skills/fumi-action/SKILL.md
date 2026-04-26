---
name: fumi-action
description: Create a new fumi action (userscript) in ~/.config/fumi/actions/ for the fumi Chrome extension
argument-hint: <what the action should do, e.g. "add a copy-PR-title button on github PRs">
---

Create a new fumi action based on the user's request.

## Request

$ARGUMENTS

## Steps

1. **Pick a filename and ID.** Lowercase kebab-case `<id>.js`. The `@id` is derived from the filename by lowercasing and replacing non-alphanumerics with `-`. If unsure, ask the user for the target site and a short slug.
2. **Choose `@match` patterns.** Use Chrome match patterns (`https://github.com/*/pull/*`, `*://*.example.com/*`, `<all_urls>`). Add `@exclude` only if needed. Confirm scope with the user when ambiguous — narrower is safer.
3. **Check for collisions.** List `~/.config/fumi/actions/` and ensure neither the filename nor the derived `@id` already exist.
4. **Write the file** to `~/.config/fumi/actions/<id>.js` with the frontmatter block first, then the action body wrapped in an IIFE. Example skeleton:

   ```js
   // ==Fumi Action==
   // @id <id>
   // @match <pattern>
   // ==/Fumi Action==

   (() => {
     // action code
   })();
   ```

5. **Tell the user how to load it.** Open the fumi extension popup → click **Reload actions** → reload the target page. Edits do not hot-reload.

## Hard rules

- Frontmatter is required. Only `@id`, `@match`, `@exclude` are recognized — any other directive causes the file to be rejected (not ignored). Directive names must be lowercase.
- `@match` must appear at least once. `@id` and `@exclude` are optional. `@match` and `@exclude` may repeat.
- The action runs in Chrome's `USER_SCRIPT` world at `document_idle`. It cannot touch page globals directly, cannot use `chrome.*` APIs, and has no persistent storage.
- SPA navigation (GitHub turbo, pjax) does not re-inject. If the target site is an SPA, hook `turbo:load` / `pjax:end` and/or use a `MutationObserver` like `actions/github-scroll-to-top.js` does.
- To call a host script, use `await fumi.run(scriptPath, payload, { timeoutMs? })`. `scriptPath` is relative to `~/.config/fumi/scripts/`, no leading `/`, no `..`. Resolves with `{ exitCode, stdout, stderr, durationMs }`; rejects only when the script never spawned. If the action needs a host script that doesn't exist yet, offer to create it via the `fumi-script` skill.
- Treat anything from the DOM as untrusted when building payloads — the page can be hostile.
- Stdout cap from `fumi.run` is 768 KiB, stderr 128 KiB; default timeout 30 s.

## Style

- Wrap the action body in an IIFE to avoid leaking globals.
- Use a stable element ID prefix (`fumi-...`) and check for existence before re-creating DOM.
- No external dependencies — these are plain scripts loaded by Chrome.
- Keep it minimal: no error handling for impossible cases, no abstractions you don't need.
