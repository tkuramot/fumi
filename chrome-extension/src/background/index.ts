// Service Worker entry point. Owns the SW lifecycle, the SW <-> User Script
// internal message router, and the few chrome.* event listeners that live
// directly in the router per §1.1-5.

import type { UserScriptMessage } from "../shared/messages.js";
import { syncActions } from "./actions.js";
import * as cm from "./chrome/contextMenus.js";
import { call } from "./chrome/nativeMessaging.js";
import * as us from "./chrome/userScripts.js";

chrome.runtime.onInstalled.addListener(async () => {
	// USER_SCRIPT world needs messaging enabled so the prelude's send() works
	// (Chrome 120+). Without this the User Script's sendMessage silently fails.
	// When "Allow User Scripts" is off, chrome.userScripts is either undefined
	// or a lingering namespace whose methods throw — swallow both shapes here
	// and let syncActions surface the typed UserScriptsDisabledError to the
	// popup instead of leaking a raw error into the SW log.
	try {
		await us.configureWorld({ messaging: true });
	} catch {
		// handled by syncActions below
	}
	await syncActions();
});

chrome.runtime.onStartup.addListener(() => {
	void syncActions();
});

// Internal router: SW <-> User Script ({kind, params}). Distinct from the
// JSON-RPC layer used by call() for SW <-> Host.
//
// Userscripts (USER_SCRIPT world, Chrome 120+) reach us via
// onUserScriptMessage; the popup uses onMessage. Register the same
// router on both so one code path handles both.
const userScriptListener = (
	msg: unknown,
	_sender: chrome.runtime.MessageSender,
	sendResponse: (r: unknown) => void,
): boolean => {
	routeUserScriptMessage(msg as UserScriptMessage)
		.then((result) => sendResponse({ result }))
		.catch((e: unknown) =>
			sendResponse({
				error: {
					message: e instanceof Error ? e.message : String(e),
					data: extractErrorData(e),
				},
			}),
		);
	return true; // keep the message channel open for async response
};

chrome.runtime.onUserScriptMessage.addListener(userScriptListener);
chrome.runtime.onMessage.addListener(userScriptListener);

chrome.contextMenus.onClicked.addListener((info, tab) => {
	if (!tab?.id) return;
	chrome.tabs.sendMessage(tab.id, {
		kind: "ctxDispatch",
		menuId: info.menuItemId,
		info,
		tab,
	});
});

async function routeUserScriptMessage(
	msg: UserScriptMessage,
): Promise<unknown> {
	switch (msg.kind) {
		case "scripts/run":
			return call("scripts/run", msg.params);

		case "contextMenus/create": {
			// Thin handler: fill defaults, delegate to the chrome wrapper.
			// Duplicate-id semantics are identical to chrome.contextMenus.create
			// (it rejects). Actions that want idempotency use remove -> create.
			const p = msg.params;
			await cm.create({
				id: p.id,
				title: p.title,
				contexts: p.contexts ?? ["page"],
			});
			return undefined;
		}

		case "contextMenus/remove":
			await cm.remove(msg.params.menuItemId);
			return undefined;

		case "refresh":
			await syncActions();
			return undefined;

		default: {
			const exhaustive: never = msg;
			throw new Error(
				`unknown kind: ${(exhaustive as { kind?: string })?.kind ?? "?"}`,
			);
		}
	}
}

function extractErrorData(e: unknown): { fumiCode?: string } | undefined {
	if (e && typeof e === "object" && "fumiCode" in e) {
		const code = (e as { fumiCode?: unknown }).fumiCode;
		if (typeof code === "string") return { fumiCode: code };
	}
	return undefined;
}
