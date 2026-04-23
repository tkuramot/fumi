// Thin wrapper over chrome.runtime.sendNativeMessage that returns a typed
// Promise and maps JSON-RPC / transport failures onto a single error surface.

import type {
	LocalErrorCode,
	Method,
	MethodParams,
	MethodResult,
	Response,
	RpcError,
} from "../../shared/protocol.js";

export const HOST_NAME = "com.tkrmt.fumi";

export class FumiHostError extends Error {
	readonly code: number;
	readonly fumiCode: string;
	readonly data: Record<string, unknown>;
	constructor(err: RpcError) {
		super(err.message);
		this.name = "FumiHostError";
		this.code = err.code;
		this.fumiCode = err.data.fumiCode;
		this.data = err.data;
	}
}

export class HostUnreachableError extends Error {
	readonly fumiCode: LocalErrorCode = "HOST_UNREACHABLE";
	constructor(reason: string) {
		super(reason);
		this.name = "HostUnreachableError";
	}
}

export async function call<M extends Method>(
	method: M,
	params?: MethodParams[M],
): Promise<MethodResult[M]> {
	const req = {
		jsonrpc: "2.0" as const,
		id: crypto.randomUUID(),
		method,
		...(params === undefined ? {} : { params }),
	};

	return new Promise<MethodResult[M]>((resolve, reject) => {
		chrome.runtime.sendNativeMessage(HOST_NAME, req, (raw: unknown) => {
			const lastError = chrome.runtime.lastError;
			if (lastError) {
				reject(
					new HostUnreachableError(lastError.message ?? "host unreachable"),
				);
				return;
			}
			const res = raw as Response<MethodResult[M]> | undefined;
			if (!res || typeof res !== "object") {
				reject(new HostUnreachableError("empty native messaging response"));
				return;
			}
			if ("error" in res) {
				reject(new FumiHostError(res.error));
				return;
			}
			resolve(res.result);
		});
	});
}
