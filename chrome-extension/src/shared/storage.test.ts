import assert from "node:assert/strict";
import { test } from "node:test";
import {
	getStatus,
	LAST_ACTIONS_KEY,
	STATUS_KEY,
	setLastActions,
	setStatus,
} from "./storage.js";
import { installChrome, makeChromeStub } from "./test-stubs/chrome.js";
import { makeStorageAreas } from "./test-stubs/storage.js";

test("setStatus / getStatus round-trip via chrome.storage.session", async (t) => {
	const storage = makeStorageAreas();
	installChrome(t, makeChromeStub({ runtime: {}, storage: storage.stub }));

	const status = { ok: true, count: 3, at: 12345 };
	await setStatus(status);
	assert.deepEqual(storage.session[STATUS_KEY], status);
	assert.deepEqual(await getStatus(), status);
});

test("getStatus returns undefined when nothing stored", async (t) => {
	const storage = makeStorageAreas();
	installChrome(t, makeChromeStub({ runtime: {}, storage: storage.stub }));

	assert.equal(await getStatus(), undefined);
});

test("setLastActions writes to chrome.storage.local", async (t) => {
	const storage = makeStorageAreas();
	installChrome(t, makeChromeStub({ runtime: {}, storage: storage.stub }));

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
	assert.deepEqual(storage.local[LAST_ACTIONS_KEY], actions);
});
