// Thin wrapper over chrome.userScripts. Promise-shaped, no fumi semantics.

export type RegisteredUserScript = chrome.userScripts.RegisteredUserScript;

export const configureWorld = (
	opts: chrome.userScripts.WorldProperties,
): Promise<void> => chrome.userScripts.configureWorld(opts);

export const list = (): Promise<RegisteredUserScript[]> =>
	chrome.userScripts.getScripts();

export const register = (scripts: RegisteredUserScript[]): Promise<void> =>
	chrome.userScripts.register(scripts);

export const unregister = (ids: string[]): Promise<void> =>
	chrome.userScripts.unregister({ ids });
