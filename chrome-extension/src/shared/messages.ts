// Internal messages between the User Script world and the Service Worker.
// Distinct from the JSON-RPC layer used for SW <-> Host.

import type { ScriptsRunParams, ScriptsRunResult } from "./protocol.js";

export type UserScriptMessage =
	| { kind: "scripts/run"; params: ScriptsRunParams }
	| { kind: "refresh" };

export type SwResponse<R = unknown> =
	| { result: R }
	| { error: { message: string; data?: { fumiCode?: string } } };

export type RunResult = ScriptsRunResult;
