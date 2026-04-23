import assert from "node:assert/strict";
import { test } from "node:test";
import {
	clearChromeStub,
	installChromeStub,
	makeChromeStub,
} from "../../shared/test-stubs/chrome.js";
import {
	call,
	FumiHostError,
	HostUnreachableError,
} from "./nativeMessaging.js";

// crypto.randomUUID exists on Node 22+ globalThis.

test("call() resolves with result on success response", async () => {
	installChromeStub(
		makeChromeStub({
			runtime: {
				sendNativeMessage: (_host, req, cb) => {
					const r = req as { id: string };
					cb({ jsonrpc: "2.0", id: r.id, result: { actions: [] } });
				},
			},
		}),
	);
	try {
		const res = await call("actions/list");
		assert.deepEqual(res, { actions: [] });
	} finally {
		clearChromeStub();
	}
});

test("call() rejects with HostUnreachableError when chrome.runtime.lastError is set", async () => {
	installChromeStub(
		makeChromeStub({
			runtime: {
				lastError: { message: "Specified native messaging host not found." },
				sendNativeMessage: (_host, _req, cb) => {
					cb(undefined);
				},
			},
		}),
	);
	try {
		await assert.rejects(
			() => call("actions/list"),
			(err: unknown) =>
				err instanceof HostUnreachableError &&
				err.fumiCode === "HOST_UNREACHABLE",
		);
	} finally {
		clearChromeStub();
	}
});

test("call() rejects with FumiHostError when response carries error envelope", async () => {
	installChromeStub(
		makeChromeStub({
			runtime: {
				sendNativeMessage: (_host, req, cb) => {
					const r = req as { id: string };
					cb({
						jsonrpc: "2.0",
						id: r.id,
						error: {
							code: -33030,
							message: "timeout",
							data: {
								fumiCode: "EXEC_TIMEOUT",
								timeoutMs: 1000,
								durationMs: 1100,
							},
						},
					});
				},
			},
		}),
	);
	try {
		await assert.rejects(
			() =>
				call("scripts/run", {
					scriptPath: "x.sh",
					payload: null,
				}),
			(err: unknown) =>
				err instanceof FumiHostError &&
				err.fumiCode === "EXEC_TIMEOUT" &&
				err.code === -33030,
		);
	} finally {
		clearChromeStub();
	}
});
