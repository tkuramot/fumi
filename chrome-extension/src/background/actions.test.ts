import assert from "node:assert/strict";
import { test } from "node:test";
import type { Action } from "../shared/protocol.js";
import { LAST_ACTIONS_KEY, STATUS_KEY } from "../shared/storage.js";
import {
	clearChromeStub,
	installChromeStub,
	makeChromeStub,
} from "../shared/test-stubs/chrome.js";
import { replaceRegisteredScripts, syncActions } from "./actions.js";

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

type SyncFixture = {
	captured: CapturedCalls;
	storage: { session: Record<string, unknown>; local: Record<string, unknown> };
};

function setupSyncChrome(opts: {
	getScriptsThrows?: boolean;
	sendNativeMessage: (
		host: string,
		req: unknown,
		cb: (res: unknown) => void,
	) => void;
}): SyncFixture {
	const captured: CapturedCalls = { unregisterIds: [], registered: [] };
	const storage = {
		session: {} as Record<string, unknown>,
		local: {} as Record<string, unknown>,
	};
	installChromeStub(
		makeChromeStub({
			runtime: {
				sendNativeMessage: opts.sendNativeMessage,
				getURL: (p: string) => `chrome-extension://stub/${p}`,
			},
			userScripts: {
				getScripts: async () => {
					if (opts.getScriptsThrows) throw new Error("user scripts disabled");
					return [];
				},
				register: async (scripts: unknown[]) => {
					captured.registered.push(scripts as Array<{ id: string }>);
				},
				unregister: async (filter: { ids: string[] }) => {
					captured.unregisterIds.push(filter.ids);
				},
			},
			storage: {
				session: {
					get: async (key: string | string[]) => {
						const k = Array.isArray(key) ? key[0] : key;
						return k && storage.session[k] !== undefined
							? { [k]: storage.session[k] }
							: {};
					},
					set: async (items: Record<string, unknown>) => {
						Object.assign(storage.session, items);
					},
				},
				local: {
					get: async () => ({}),
					set: async (items: Record<string, unknown>) => {
						Object.assign(storage.local, items);
					},
				},
			},
		}),
	);
	return { captured, storage };
}

// syncActions calls getPrelude() which fetches a chrome-extension:// URL.
// Stub global fetch so the request resolves with a fake body.
function stubFetch(): () => void {
	const original = globalThis.fetch;
	(globalThis as unknown as { fetch: typeof fetch }).fetch = (async () => ({
		text: async () => "/* prelude */",
	})) as unknown as typeof fetch;
	return () => {
		(globalThis as unknown as { fetch: typeof fetch | undefined }).fetch =
			original;
	};
}

test("syncActions() persists ok status and last actions on success", async () => {
	const restoreFetch = stubFetch();
	const action: Action = {
		id: "a",
		path: "a.js",
		matches: ["https://x/*"],
		excludes: [],
		code: "",
	};
	const fx = setupSyncChrome({
		sendNativeMessage: (_host, req, cb) => {
			const r = req as { id: string };
			cb({ jsonrpc: "2.0", id: r.id, result: { actions: [action] } });
		},
	});
	try {
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
	} finally {
		clearChromeStub();
		restoreFetch();
	}
});

test("syncActions() records error status when host rejects", async () => {
	const fx = setupSyncChrome({
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
	try {
		await syncActions();
		const status = fx.storage.session[STATUS_KEY] as {
			ok: boolean;
			error: string;
		};
		assert.equal(status.ok, false);
		assert.match(status.error, /store missing/);
		assert.equal(fx.captured.registered.length, 0);
	} finally {
		clearChromeStub();
	}
});

test("syncActions() surfaces UserScriptsDisabledError when getScripts throws", async () => {
	const fx = setupSyncChrome({
		getScriptsThrows: true,
		sendNativeMessage: () => {
			throw new Error("should not be called");
		},
	});
	try {
		await syncActions();
		const status = fx.storage.session[STATUS_KEY] as {
			ok: boolean;
			error: string;
		};
		assert.equal(status.ok, false);
		assert.match(status.error, /User Scripts API is disabled/);
	} finally {
		clearChromeStub();
	}
});
