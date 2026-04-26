// Minimal, hand-written chrome.* stub used by node:test suites.
// Each factory helper returns a partial chrome object; tests compose them as
// needed and assign the result to globalThis.chrome.

import type { TestContext } from "node:test";

type AnyFn = (...args: unknown[]) => unknown;

export type RuntimeStub = {
	lastError?: { message: string };
	sendNativeMessage: (
		host: string,
		msg: unknown,
		cb: (res: unknown) => void,
	) => void;
	sendMessage: (msg: unknown, cb?: (res: unknown) => void) => void;
	onMessage: { addListener: (fn: AnyFn) => void };
	onInstalled: { addListener: (fn: AnyFn) => void };
	onStartup: { addListener: (fn: AnyFn) => void };
	getURL: (path: string) => string;
	id?: string;
};

export type UserScriptsStub = {
	configureWorld: (opts: unknown) => Promise<void>;
	getScripts: () => Promise<Array<{ id: string }>>;
	register: (scripts: unknown[]) => Promise<void>;
	unregister: (filter: { ids: string[] }) => Promise<void>;
};

export type ContextMenusStub = {
	create: (props: unknown, cb?: () => void) => void;
	remove: (id: string | number, cb?: () => void) => void;
	onClicked: { addListener: (fn: AnyFn) => void };
};

export type StorageAreaStub = {
	get: (key: string | string[]) => Promise<Record<string, unknown>>;
	set: (items: Record<string, unknown>) => Promise<void>;
};

export type ChromeStub = {
	runtime: Partial<RuntimeStub>;
	userScripts?: Partial<UserScriptsStub>;
	contextMenus?: Partial<ContextMenusStub>;
	storage?: {
		session?: Partial<StorageAreaStub>;
		local?: Partial<StorageAreaStub>;
	};
	tabs?: {
		sendMessage: (tabId: number, msg: unknown) => void;
	};
};

export function makeChromeStub(
	overrides: ChromeStub = { runtime: {} },
): ChromeStub {
	const { runtime: runtimeOverride, ...rest } = overrides;
	return {
		...rest,
		runtime: {
			sendNativeMessage: () => {},
			sendMessage: () => {},
			onMessage: { addListener: () => {} },
			onInstalled: { addListener: () => {} },
			onStartup: { addListener: () => {} },
			getURL: (p: string) => `chrome-extension://stub/${p}`,
			...runtimeOverride,
		},
	};
}

export function installChromeStub(stub: ChromeStub): void {
	(globalThis as unknown as { chrome: unknown }).chrome = stub;
}

export function clearChromeStub(): void {
	delete (globalThis as unknown as { chrome?: unknown }).chrome;
}

// Installs the stub and registers cleanup on the test context, so individual
// tests don't need try/finally blocks around their bodies.
export function installChrome(t: TestContext, stub: ChromeStub): void {
	installChromeStub(stub);
	t.after(clearChromeStub);
}

// Stubs globalThis.fetch with a function that returns `{ text: async () => body }`,
// and restores the original fetch after the test completes.
export function stubFetch(t: TestContext, body = ""): void {
	const original = globalThis.fetch;
	(globalThis as unknown as { fetch: typeof fetch }).fetch = (async () => ({
		text: async () => body,
	})) as unknown as typeof fetch;
	t.after(() => {
		(globalThis as unknown as { fetch: typeof fetch | undefined }).fetch =
			original;
	});
}
