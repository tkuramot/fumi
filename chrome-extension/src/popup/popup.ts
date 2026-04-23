// Popup: reads status from chrome.storage.session and offers a manual
// resync. Intentionally minimal — no framework, no CRUD UI.

import { getStatus, type Status } from "../shared/storage.js";

async function render(): Promise<void> {
	const status: Status | undefined = await getStatus();
	const $ = (id: string): HTMLElement => {
		const el = document.getElementById(id);
		if (!el) throw new Error(`missing #${id}`);
		return el;
	};

	if (!status) {
		$("status").textContent = "not yet fetched";
		$("count").textContent = "—";
		$("at").textContent = "—";
		$("error").textContent = "—";
		return;
	}

	$("status").textContent = status.ok ? "OK" : "error";
	$("status").className = status.ok ? "ok" : "err";
	$("count").textContent =
		status.count !== undefined ? String(status.count) : "—";
	$("at").textContent = status.at ? new Date(status.at).toLocaleString() : "—";
	$("error").textContent = status.error ?? "—";
}

document.getElementById("resync")?.addEventListener("click", async () => {
	const btn = document.getElementById("resync") as HTMLButtonElement;
	btn.disabled = true;
	try {
		await chrome.runtime.sendMessage({ kind: "resync" });
	} finally {
		btn.disabled = false;
		await render();
	}
});

void render();
