// Minimal, hand-written chrome.* stub used by node:test suites.
// Each factory helper returns a partial chrome object; tests compose them as
// needed and assign the result to globalThis.chrome.

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
	remove: (id: string | number) => Promise<void>;
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
