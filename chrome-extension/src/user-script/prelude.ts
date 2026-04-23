// User Script prelude. Injected ahead of every action's code.
//
// Constraint: this file has NO imports and must stand alone once compiled,
// because the build has no bundler — tsc emits this file as-is and the SW
// fetches it verbatim from the extension URL.

(() => {
	type Send = <R>(kind: string, params?: unknown) => Promise<R>;

	// Internal envelope for SW <-> User Script. The SW translates this into
	// JSON-RPC for the Host when required.
	const send: Send = <R>(kind: string, params?: unknown) =>
		new Promise<R>((resolve, reject) => {
			chrome.runtime.sendMessage({ kind, params }, (res: unknown) => {
				const lastError = chrome.runtime.lastError;
				if (lastError) {
					reject(new Error(lastError.message ?? "sendMessage failed"));
					return;
				}
				const r = res as
					| { result?: R }
					| {
							error?: {
								message?: string;
								data?: { fumiCode?: string };
							};
					  }
					| undefined;
				if (r && "error" in r && r.error) {
					reject(
						new Error(r.error.data?.fumiCode ?? r.error.message ?? "UNKNOWN"),
					);
					return;
				}
				resolve((r as { result: R } | undefined)?.result as R);
			});
		});

	type CtxHandler = (
		info: chrome.contextMenus.OnClickData,
		tab?: chrome.tabs.Tab,
	) => void;

	const ctxHandlers = new Map<string | number, CtxHandler>();

	chrome.runtime.onMessage.addListener((msg: unknown) => {
		const m = msg as
			| {
					kind?: string;
					menuId?: string | number;
					info?: chrome.contextMenus.OnClickData;
					tab?: chrome.tabs.Tab;
			  }
			| undefined;
		if (m?.kind === "ctxDispatch" && m.menuId !== undefined && m.info) {
			ctxHandlers.get(m.menuId)?.(m.info, m.tab);
		}
	});

	(globalThis as unknown as { fumi: unknown }).fumi = {
		run: (
			scriptPath: string,
			payload: unknown,
			opts?: { timeoutMs?: number },
		) =>
			send("scripts/run", {
				scriptPath,
				payload,
				...(opts ?? {}),
			}),

		contextMenus: {
			// Mirrors chrome.contextMenus.create. Duplicate-id behavior matches
			// chrome.* — callers use remove -> create for idempotency.
			create: (props: {
				id: string;
				title: string;
				contexts?: chrome.contextMenus.ContextType[];
				onClicked: CtxHandler;
			}) => {
				ctxHandlers.set(props.id, props.onClicked);
				return send("contextMenus/create", {
					id: props.id,
					title: props.title,
					contexts: props.contexts,
				});
			},
			remove: (menuItemId: string | number) => {
				ctxHandlers.delete(menuItemId);
				return send("contextMenus/remove", { menuItemId });
			},
		},
	};
})();
