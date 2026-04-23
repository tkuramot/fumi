import type { Action } from "./protocol.js";

// Keys written to chrome.storage. Only cache + status data lives here —
// never configuration or script bodies.

export type Status = {
	ok: boolean;
	count?: number;
	error?: string;
	at: number;
};

export const STATUS_KEY = "status";
export const LAST_ACTIONS_KEY = "lastActions";

export async function setStatus(status: Status): Promise<void> {
	await chrome.storage.session.set({ [STATUS_KEY]: status });
}

export async function getStatus(): Promise<Status | undefined> {
	const out = await chrome.storage.session.get(STATUS_KEY);
	return out[STATUS_KEY] as Status | undefined;
}

export async function setLastActions(actions: Action[]): Promise<void> {
	await chrome.storage.local.set({ [LAST_ACTIONS_KEY]: actions });
}
