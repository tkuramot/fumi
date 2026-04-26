import assert from "node:assert/strict";
import { test } from "node:test";
import {
	installChrome,
	makeChromeStub,
} from "../../shared/test-stubs/chrome.js";
import { create, remove } from "./contextMenus.js";

test("create() resolves when chrome reports no lastError", async (t) => {
	let captured: unknown;
	installChrome(
		t,
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

	await create({ id: "x", title: "X" });
	assert.deepEqual(captured, { id: "x", title: "X" });
});

test("create() rejects with lastError message", async (t) => {
	installChrome(
		t,
		makeChromeStub({
			runtime: { lastError: { message: "duplicate id" } },
			contextMenus: {
				create: (_props: unknown, cb?: () => void) => {
					cb?.();
				},
			},
		}),
	);

	await assert.rejects(() => create({ id: "x", title: "X" }), /duplicate id/);
});

test("remove() resolves when chrome reports no lastError", async (t) => {
	let removedId: string | number | undefined;
	installChrome(
		t,
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

	await remove("y");
	assert.equal(removedId, "y");
});

test("remove() rejects with lastError message", async (t) => {
	installChrome(
		t,
		makeChromeStub({
			runtime: { lastError: { message: "no such menu" } },
			contextMenus: {
				remove: (_id: string | number, cb?: () => void) => {
					cb?.();
				},
			},
		}),
	);

	await assert.rejects(() => remove("y"), /no such menu/);
});
