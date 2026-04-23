// Wire-level types for the fumi Native Messaging JSON-RPC 2.0 protocol.
// Single source of truth on the TS side — keep in sync with internal/protocol
// on the Go side and docs/design/protocol.md.

export type JsonRpcId = string;

export type ActionsListRequest = {
	jsonrpc: "2.0";
	id: JsonRpcId;
	method: "actions/list";
	params?: undefined;
};

export type ScriptsRunRequest = {
	jsonrpc: "2.0";
	id: JsonRpcId;
	method: "scripts/run";
	params: ScriptsRunParams;
};

export type ScriptsRunParams = {
	scriptPath: string;
	payload: unknown;
	timeoutMs?: number;
	context?: Record<string, string>;
};

export type Request = ActionsListRequest | ScriptsRunRequest;

export type Action = {
	id: string;
	path: string;
	matches: string[];
	excludes: string[];
	code: string;
};

export type ActionsListResult = { actions: Action[] };

export type ScriptsRunResult = {
	exitCode: number;
	stdout: string;
	stderr: string;
	durationMs: number;
};

export type MethodResult = {
	"actions/list": ActionsListResult;
	"scripts/run": ScriptsRunResult;
};

export type MethodParams = {
	"actions/list": undefined;
	"scripts/run": ScriptsRunParams;
};

export type Method = keyof MethodResult;

export type FumiErrorCode =
	| "PROTO_PARSE_ERROR"
	| "PROTO_INVALID_REQUEST"
	| "PROTO_METHOD_NOT_FOUND"
	| "PROTO_INVALID_PARAMS"
	| "INTERNAL"
	| "STORE_NOT_FOUND"
	| "STORE_CONFIG_INVALID"
	| "STORE_ACTIONS_TOO_LARGE"
	| "STORE_FRONTMATTER_INVALID"
	| "SCRIPT_INVALID_PATH"
	| "SCRIPT_NOT_FOUND"
	| "SCRIPT_NOT_REGULAR_FILE"
	| "SCRIPT_NOT_EXECUTABLE"
	| "EXEC_TIMEOUT"
	| "EXEC_OUTPUT_TOO_LARGE"
	| "EXEC_SPAWN_FAILED";

export type RpcError = {
	code: number;
	message: string;
	data: { fumiCode: FumiErrorCode } & Record<string, unknown>;
};

export type Response<R> =
	| { jsonrpc: "2.0"; id: JsonRpcId; result: R }
	| { jsonrpc: "2.0"; id: JsonRpcId; error: RpcError };

// Locally-classified error that never appears on the wire.
export type LocalErrorCode = "HOST_UNREACHABLE";
