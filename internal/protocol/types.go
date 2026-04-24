package protocol

import "encoding/json"

const JsonRpcVersion = "2.0"

const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternal       = -32603
)

type Request struct {
	JsonRpc string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JsonRpc string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RpcError       `json:"error,omitempty"`
}

type RpcError struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

func (e *RpcError) Error() string { return e.Message }

type Action struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Matches  []string `json:"matches"`
	Excludes []string `json:"excludes"`
	Code     string   `json:"code"`
}

type GetActionsResult struct {
	Actions []Action `json:"actions"`
}

type RunScriptParams struct {
	ScriptPath string          `json:"scriptPath"`
	Payload    json.RawMessage `json:"payload"`
	TimeoutMs  *int            `json:"timeoutMs,omitempty"`
}

type RunScriptResult struct {
	ExitCode   int    `json:"exitCode"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"durationMs"`
}
