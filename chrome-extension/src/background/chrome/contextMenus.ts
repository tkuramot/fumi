// Thin wrapper over chrome.contextMenus. Holds no fumi semantics.
// Export names mirror the chrome.* methods (naming rule §4.1-7).

export const create = (
	props: chrome.contextMenus.CreateProperties,
): Promise<void> =>
	new Promise((resolve, reject) => {
		chrome.contextMenus.create(props, () => {
			const err = chrome.runtime.lastError;
			if (err) {
				reject(new Error(err.message));
				return;
			}
			resolve();
		});
	});

export const remove = (menuItemId: string | number): Promise<void> =>
	new Promise((resolve, reject) => {
		chrome.contextMenus.remove(menuItemId, () => {
			const err = chrome.runtime.lastError;
			if (err) {
				reject(new Error(err.message));
				return;
			}
			resolve();
		});
	});
