import assert from "node:assert/strict";
import { test } from "node:test";
import {
	getStatus,
	LAST_ACTIONS_KEY,
	STATUS_KEY,
	setLastActions,
	setStatus,
} from "./storage.js";
import {
	clearChromeStub,
	installChromeStub,
	makeChromeStub,
} from "./test-stubs/chrome.js";

function makeStorageStub() {
	const session: Record<string, unknown> = {};
	const local: Record<string, unknown> = {};
	installChromeStub(
		makeChromeStub({
			runtime: {},
			storage: {
				session: {
					get: async (key: string | string[]) => {
						const k = Array.isArray(key) ? key[0] : key;
						return k && session[k] !== undefined ? { [k]: session[k] } : {};
					},
					set: async (items: Record<string, unknown>) => {
						Object.assign(session, items);
					},
				},
				local: {
					get: async () => ({}),
					set: async (items: Record<string, unknown>) => {
						Object.assign(local, items);
					},
				},
			},
		}),
	);
	return { session, local };
}

test("setStatus / getStatus round-trip via chrome.storage.session", async () => {
	const { session } = makeStorageStub();
	try {
		const status = { ok: true, count: 3, at: 12345 };
		await setStatus(status);
		assert.deepEqual(session[STATUS_KEY], status);
		assert.deepEqual(await getStatus(), status);
	} finally {
		clearChromeStub();
	}
});

test("getStatus returns undefined when nothing stored", async () => {
	makeStorageStub();
	try {
		assert.equal(await getStatus(), undefined);
	} finally {
		clearChromeStub();
	}
});

test("setLastActions writes to chrome.storage.local", async () => {
	const { local } = makeStorageStub();
	try {
		const actions = [
			{
				id: "a",
				path: "a.js",
				matches: [],
				excludes: [],
				code: "",
			},
		];
		await setLastActions(actions);
		assert.deepEqual(local[LAST_ACTIONS_KEY], actions);
	} finally {
		clearChromeStub();
	}
});
