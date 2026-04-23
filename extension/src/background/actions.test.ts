import assert from "node:assert/strict";
import { test } from "node:test";
import type { Action } from "../shared/protocol.js";
import {
	clearChromeStub,
	installChromeStub,
	makeChromeStub,
} from "../shared/test-stubs/chrome.js";
import { replaceRegisteredScripts } from "./actions.js";

type CapturedCalls = {
	unregisterIds: string[][];
	registered: Array<Array<{ id: string }>>;
};

function setupChrome(existing: Array<{ id: string }>): CapturedCalls {
	const captured: CapturedCalls = { unregisterIds: [], registered: [] };
	installChromeStub(
		makeChromeStub({
			runtime: {},
			userScripts: {
				getScripts: async () => existing,
				register: async (scripts: unknown[]) => {
					captured.registered.push(scripts as Array<{ id: string }>);
				},
				unregister: async (filter: { ids: string[] }) => {
					captured.unregisterIds.push(filter.ids);
				},
			},
		}),
	);
	return captured;
}

test("replaceRegisteredScripts() unregisters existing then registers all new actions", async () => {
	const captured = setupChrome([{ id: "fumi:old-a" }, { id: "fumi:old-b" }]);
	try {
		const actions: Action[] = [
			{
				id: "save-note",
				path: "save-note.js",
				matches: ["https://example.com/*"],
				excludes: [],
				code: "console.log('hi');",
			},
		];
		await replaceRegisteredScripts(actions, "/* prelude */");

		assert.deepEqual(captured.unregisterIds, [["fumi:old-a", "fumi:old-b"]]);
		assert.equal(captured.registered.length, 1);
		const [reg] = captured.registered[0] as unknown as Array<{
			id: string;
			js: Array<{ code: string }>;
			world: string;
		}>;
		assert.equal(reg.id, "fumi:save-note");
		assert.equal(reg.world, "USER_SCRIPT");
		assert.equal(reg.js[0]?.code, "/* prelude */");
		assert.equal(reg.js[1]?.code, "console.log('hi');");
	} finally {
		clearChromeStub();
	}
});

test("replaceRegisteredScripts() skips unregister when none exist and skips register when no actions", async () => {
	const captured = setupChrome([]);
	try {
		await replaceRegisteredScripts([], "/* prelude */");
		assert.equal(captured.unregisterIds.length, 0);
		assert.equal(captured.registered.length, 0);
	} finally {
		clearChromeStub();
	}
});
