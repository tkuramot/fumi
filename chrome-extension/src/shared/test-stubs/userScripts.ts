// chrome.userScripts fake that captures register/unregister calls so tests can
// assert on what the SW asked Chrome to install.

import type { ChromeStub } from "./chrome.js";

export type UserScriptsCapture = {
	unregisterIds: string[][];
	registered: Array<Array<{ id: string }>>;
};

export type UserScriptsFixture = {
	stub: NonNullable<ChromeStub["userScripts"]>;
	captured: UserScriptsCapture;
};

export function makeUserScriptsCapture(
	opts: { existing?: Array<{ id: string }>; getScriptsThrows?: boolean } = {},
): UserScriptsFixture {
	const captured: UserScriptsCapture = {
		unregisterIds: [],
		registered: [],
	};
	return {
		captured,
		stub: {
			getScripts: async () => {
				if (opts.getScriptsThrows) throw new Error("user scripts disabled");
				return opts.existing ?? [];
			},
			register: async (scripts: unknown[]) => {
				captured.registered.push(scripts as Array<{ id: string }>);
			},
			unregister: async (filter: { ids: string[] }) => {
				captured.unregisterIds.push(filter.ids);
			},
		},
	};
}
