import assert from "node:assert/strict";
import { type TestContext, test } from "node:test";
import type { Action } from "../shared/protocol.js";
import { LAST_ACTIONS_KEY, STATUS_KEY } from "../shared/storage.js";
import {
	installChrome,
	makeChromeStub,
	stubFetch,
} from "../shared/test-stubs/chrome.js";
import { makeStorageAreas } from "../shared/test-stubs/storage.js";
import { makeUserScriptsCapture } from "../shared/test-stubs/userScripts.js";
import { replaceRegisteredScripts, syncActions } from "./actions.js";

test("replaceRegisteredScripts() unregisters existing then registers all new actions", async (t) => {
	const us = makeUserScriptsCapture({
		existing: [{ id: "fumi:old-a" }, { id: "fumi:old-b" }],
	});
	installChrome(t, makeChromeStub({ runtime: {}, userScripts: us.stub }));

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

	assert.deepEqual(us.captured.unregisterIds, [["fumi:old-a", "fumi:old-b"]]);
	assert.equal(us.captured.registered.length, 1);
	const [reg] = us.captured.registered[0] as unknown as Array<{
		id: string;
		js: Array<{ code: string }>;
		world: string;
	}>;
	assert.equal(reg.id, "fumi:save-note");
	assert.equal(reg.world, "USER_SCRIPT");
	assert.equal(reg.js[0]?.code, "/* prelude */");
	assert.equal(reg.js[1]?.code, "console.log('hi');");
});

test("replaceRegisteredScripts() skips unregister when none exist and skips register when no actions", async (t) => {
	const us = makeUserScriptsCapture();
	installChrome(t, makeChromeStub({ runtime: {}, userScripts: us.stub }));

	await replaceRegisteredScripts([], "/* prelude */");
	assert.equal(us.captured.unregisterIds.length, 0);
	assert.equal(us.captured.registered.length, 0);
});

function installSyncStub(
	t: TestContext,
	opts: {
		getScriptsThrows?: boolean;
		sendNativeMessage: (
			host: string,
			req: unknown,
			cb: (res: unknown) => void,
		) => void;
	},
) {
	const us = makeUserScriptsCapture({
		getScriptsThrows: opts.getScriptsThrows,
	});
	const storage = makeStorageAreas();
	installChrome(
		t,
		makeChromeStub({
			runtime: {
				sendNativeMessage: opts.sendNativeMessage,
				getURL: (p: string) => `chrome-extension://stub/${p}`,
			},
			userScripts: us.stub,
			storage: storage.stub,
		}),
	);
	return { captured: us.captured, storage };
}

test("syncActions() persists ok status and last actions on success", async (t) => {
	stubFetch(t, "/* prelude */");
	const action: Action = {
		id: "a",
		path: "a.js",
		matches: ["https://x/*"],
		excludes: [],
		code: "",
	};
	const fx = installSyncStub(t, {
		sendNativeMessage: (_host, req, cb) => {
			const r = req as { id: string };
			cb({ jsonrpc: "2.0", id: r.id, result: { actions: [action] } });
		},
	});

	await syncActions();
	const status = fx.storage.session[STATUS_KEY] as {
		ok: boolean;
		count: number;
		error?: string;
	};
	assert.equal(status.ok, true);
	assert.equal(status.count, 1);
	assert.equal(status.error, undefined);
	assert.deepEqual(fx.storage.local[LAST_ACTIONS_KEY], [action]);
	assert.equal(fx.captured.registered.length, 1);
});

test("syncActions() records error status when host rejects", async (t) => {
	const fx = installSyncStub(t, {
		sendNativeMessage: (_host, req, cb) => {
			const r = req as { id: string };
			cb({
				jsonrpc: "2.0",
				id: r.id,
				error: {
					code: -33001,
					message: "store missing",
					data: { fumiCode: "STORE_NOT_FOUND" },
				},
			});
		},
	});

	await syncActions();
	const status = fx.storage.session[STATUS_KEY] as {
		ok: boolean;
		error: string;
	};
	assert.equal(status.ok, false);
	assert.match(status.error, /store missing/);
	assert.equal(fx.captured.registered.length, 0);
});

test("syncActions() surfaces UserScriptsDisabledError when getScripts throws", async (t) => {
	const fx = installSyncStub(t, {
		getScriptsThrows: true,
		sendNativeMessage: () => {
			throw new Error("should not be called");
		},
	});

	await syncActions();
	const status = fx.storage.session[STATUS_KEY] as {
		ok: boolean;
		error: string;
	};
	assert.equal(status.ok, false);
	assert.match(status.error, /User Scripts API is disabled/);
});
