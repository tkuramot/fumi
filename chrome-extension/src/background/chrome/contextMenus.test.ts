import assert from "node:assert/strict";
import { test } from "node:test";
import {
	clearChromeStub,
	installChromeStub,
	makeChromeStub,
} from "../../shared/test-stubs/chrome.js";
import { create, remove } from "./contextMenus.js";

test("create() resolves when chrome reports no lastError", async () => {
	let captured: unknown;
	installChromeStub(
		makeChromeStub({
			runtime: {},
			contextMenus: {
				create: (props: unknown, cb?: () => void) => {
					captured = props;
					cb?.();
				},
			},
		}),
	);
	try {
		await create({ id: "x", title: "X" });
		assert.deepEqual(captured, { id: "x", title: "X" });
	} finally {
		clearChromeStub();
	}
});

test("create() rejects with lastError message", async () => {
	installChromeStub(
		makeChromeStub({
			runtime: { lastError: { message: "duplicate id" } },
			contextMenus: {
				create: (_props: unknown, cb?: () => void) => {
					cb?.();
				},
			},
		}),
	);
	try {
		await assert.rejects(() => create({ id: "x", title: "X" }), /duplicate id/);
	} finally {
		clearChromeStub();
	}
});

test("remove() resolves when chrome reports no lastError", async () => {
	let removedId: string | number | undefined;
	installChromeStub(
		makeChromeStub({
			runtime: {},
			contextMenus: {
				remove: (id: string | number, cb?: () => void) => {
					removedId = id;
					cb?.();
				},
			},
		}),
	);
	try {
		await remove("y");
		assert.equal(removedId, "y");
	} finally {
		clearChromeStub();
	}
});

test("remove() rejects with lastError message", async () => {
	installChromeStub(
		makeChromeStub({
			runtime: { lastError: { message: "no such menu" } },
			contextMenus: {
				remove: (_id: string | number, cb?: () => void) => {
					cb?.();
				},
			},
		}),
	);
	try {
		await assert.rejects(() => remove("y"), /no such menu/);
	} finally {
		clearChromeStub();
	}
});
