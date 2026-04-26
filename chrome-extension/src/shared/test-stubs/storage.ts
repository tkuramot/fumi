// In-memory chrome.storage.session / chrome.storage.local fake.
// Returns the partial stub plus the underlying records so tests can assert on writes.

import type { ChromeStub } from "./chrome.js";

export type StorageAreasFixture = {
	stub: NonNullable<ChromeStub["storage"]>;
	session: Record<string, unknown>;
	local: Record<string, unknown>;
};

export function makeStorageAreas(): StorageAreasFixture {
	const session: Record<string, unknown> = {};
	const local: Record<string, unknown> = {};
	const area = (backing: Record<string, unknown>) => ({
		get: async (key: string | string[]) => {
			const k = Array.isArray(key) ? key[0] : key;
			return k && backing[k] !== undefined ? { [k]: backing[k] } : {};
		},
		set: async (items: Record<string, unknown>) => {
			Object.assign(backing, items);
		},
	});
	return {
		stub: { session: area(session), local: area(local) },
		session,
		local,
	};
}
