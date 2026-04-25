// Action sync domain file. Orchestrates host call + userscript replacement
// + status write. Promoted from the router per §1.1 because it exceeds the
// "thin passthrough" threshold.

import type { Action } from "../shared/protocol.js";
import { setLastActions, setStatus } from "../shared/storage.js";
import { call } from "./chrome/nativeMessaging.js";
import * as us from "./chrome/userScripts.js";

let preludeCache: string | null = null;

// Fetch prelude.js from the extension bundle. The userscript prelude is a
// self-contained file; chrome.runtime.getURL + fetch works from the SW.
async function getPrelude(): Promise<string> {
	if (preludeCache !== null) return preludeCache;
	const res = await fetch(chrome.runtime.getURL("userscript/prelude.js"));
	preludeCache = await res.text();
	return preludeCache;
}

export async function syncActions(): Promise<void> {
	try {
		await assertUserScriptsAvailable();
		const { actions } = await call("actions/list");
		await replaceRegisteredScripts(actions);
		await setLastActions(actions);
		await setStatus({ ok: true, count: actions.length, at: Date.now() });
	} catch (e) {
		await setStatus({ ok: false, error: errorMessage(e), at: Date.now() });
	}
}

class UserScriptsDisabledError extends Error {
	constructor() {
		super(
			'User Scripts API is disabled. Open chrome://extensions, find "fumi", and enable the "Allow User Scripts" toggle. fumi will reload automatically.',
		);
		this.name = "UserScriptsDisabledError";
	}
}

// The User Scripts API is gated by the per-extension "Allow User Scripts"
// toggle. When off, chrome.userScripts is either undefined or a lingering
// namespace whose methods throw on call. Probe getScripts() so both shapes
// collapse into one typed error.
async function assertUserScriptsAvailable(): Promise<void> {
	try {
		await chrome.userScripts.getScripts();
	} catch {
		throw new UserScriptsDisabledError();
	}
}

export async function replaceRegisteredScripts(
	actions: Action[],
	prelude?: string,
): Promise<void> {
	const existing = await us.list();
	if (existing.length > 0) {
		await us.unregister(existing.map((s) => s.id));
	}
	if (actions.length === 0) return;

	const preludeJs = prelude ?? (await getPrelude());
	await us.register(
		actions.map((a) => ({
			id: `fumi:${a.id}`,
			matches: a.matches,
			excludeMatches: a.excludes,
			world: "USER_SCRIPT",
			runAt: "document_idle",
			js: [{ code: preludeJs }, { code: a.code }],
		})),
	);
}

function errorMessage(e: unknown): string {
	if (e instanceof Error) return e.message;
	return String(e);
}
