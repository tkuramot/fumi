// Popup: reads status from chrome.storage.session and offers a manual
// refresh. Intentionally minimal — no framework, no CRUD UI.

import { getStatus, type Status } from "../shared/storage.js";

function $(id: string): HTMLElement {
	const el = document.getElementById(id);
	if (!el) throw new Error(`missing #${id}`);
	return el;
}

function relativeTime(ts: number): string {
	const diff = Date.now() - ts;
	if (diff < 0) return "just now";
	const sec = Math.floor(diff / 1000);
	if (sec < 5) return "just now";
	if (sec < 60) return `${sec}s ago`;
	const min = Math.floor(sec / 60);
	if (min < 60) return `${min}m ago`;
	const hr = Math.floor(min / 60);
	if (hr < 24) return `${hr}h ago`;
	const day = Math.floor(hr / 24);
	return `${day}d ago`;
}

function setStatusBadge(state: "ok" | "err" | "idle", text: string): void {
	const el = $("status");
	el.textContent = text;
	el.className = `badge${state === "idle" ? "" : ` ${state}`}`;
}

// Strip a leading "v"/whitespace so "0.1.9" and "v0.1.9" compare equal.
function normalizeVersion(v: string): string {
	return v.trim().replace(/^v/, "");
}

// True if versions differ. "dev"/empty host = dev build, never warn.
function versionsMismatch(ext: string, host: string | undefined): boolean {
	if (!host) return false;
	const h = normalizeVersion(host);
	if (h === "" || h === "dev") return false;
	return normalizeVersion(ext) !== h;
}

function renderUpdateWarning(extVersion: string, hostVersion?: string): void {
	const el = $("update-warning");
	if (!versionsMismatch(extVersion, hostVersion)) {
		el.classList.remove("visible");
		el.textContent = "";
		return;
	}
	el.replaceChildren(
		document.createTextNode(
			`Version mismatch — extension v${normalizeVersion(extVersion)}, host v${normalizeVersion(hostVersion ?? "")}. Update the host with `,
		),
		Object.assign(document.createElement("code"), {
			textContent: "brew upgrade --cask fumi",
		}),
		document.createTextNode(", then reload the extension."),
	);
	el.classList.add("visible");
}

async function render(): Promise<void> {
	const extVersion = chrome.runtime.getManifest().version;
	$("version").textContent = `v${extVersion}`;

	const status: Status | undefined = await getStatus();
	const errEl = $("error");

	renderUpdateWarning(extVersion, status?.hostVersion);

	if (!status) {
		setStatusBadge("idle", "not fetched");
		$("count").textContent = "—";
		$("at").textContent = "—";
		$("at").removeAttribute("title");
		errEl.classList.remove("visible");
		errEl.textContent = "";
		return;
	}

	setStatusBadge(status.ok ? "ok" : "err", status.ok ? "OK" : "Error");
	$("count").textContent =
		status.count !== undefined ? String(status.count) : "—";

	if (status.at) {
		$("at").textContent = relativeTime(status.at);
		$("at").title = new Date(status.at).toLocaleString();
	} else {
		$("at").textContent = "—";
		$("at").removeAttribute("title");
	}

	if (status.error) {
		errEl.textContent = status.error;
		errEl.classList.add("visible");
	} else {
		errEl.classList.remove("visible");
		errEl.textContent = "";
	}
}

document.getElementById("refresh")?.addEventListener("click", async () => {
	const btn = document.getElementById("refresh") as HTMLButtonElement;
	const original = btn.textContent;
	btn.disabled = true;
	btn.textContent = "Refreshing…";
	try {
		await chrome.runtime.sendMessage({ kind: "refresh" });
	} finally {
		btn.disabled = false;
		btn.textContent = original;
		await render();
	}
});

void render();
