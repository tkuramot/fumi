// Internal messages between the User Script world and the Service Worker.
// Distinct from the JSON-RPC layer used for SW <-> Host.

import type { ScriptsRunParams, ScriptsRunResult } from "./protocol.js";

export type UserScriptMessage =
	| { kind: "scripts/run"; params: ScriptsRunParams }
	| {
			kind: "contextMenus/create";
			params: {
				id: string;
				title: string;
				contexts?: chrome.contextMenus.ContextType[];
			};
	  }
	| { kind: "contextMenus/remove"; params: { menuItemId: string | number } }
	| { kind: "refresh" };

// SW -> User Script dispatch for chrome.contextMenus.onClicked.
export type CtxDispatchMessage = {
	kind: "ctxDispatch";
	menuId: string | number;
	info: chrome.contextMenus.OnClickData;
	tab?: chrome.tabs.Tab;
};

export type SwResponse<R = unknown> =
	| { result: R }
	| { error: { message: string; data?: { fumiCode?: string } } };

export type RunResult = ScriptsRunResult;
