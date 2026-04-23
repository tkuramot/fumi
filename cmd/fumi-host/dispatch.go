package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/tkuramot/fumi/internal/config"
	"github.com/tkuramot/fumi/internal/protocol"
	"github.com/tkuramot/fumi/internal/store"
)

// nullID is the JSON value used for id when the request id is unknown
// (e.g. parse errors). Per JSON-RPC 2.0 §4.2.
var nullID = json.RawMessage("null")

func run(stdin io.Reader, stdout, stderr io.Writer) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "fumi-host: panic: %v\n", r)
			writeErrorObj(stdout, nullID, protocol.NewError(
				"INTERNAL", fmt.Sprintf("panic: %v", r), nil))
			code = 0
		}
	}()

	body, err := protocol.ReadMessage(stdin)
	if err != nil {
		if err == io.EOF {
			// Chrome closed the pipe without sending anything; nothing to reply.
			return 0
		}
		writeErrorObj(stdout, nullID, protocol.NewError(
			"PROTO_PARSE_ERROR", err.Error(), nil))
		return 0
	}

	// Batch requests are arrays — not supported in this short-lived protocol.
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	if len(trimmed) > 0 && trimmed[0] == '[' {
		writeErrorObj(stdout, nullID, protocol.NewError(
			"PROTO_INVALID_REQUEST", "batch requests are not supported", nil))
		return 0
	}

	var req protocol.Request
	if err := json.Unmarshal(body, &req); err != nil {
		writeErrorObj(stdout, nullID, protocol.NewError(
			"PROTO_PARSE_ERROR", err.Error(), nil))
		return 0
	}

	// Notifications (no id) receive no response per JSON-RPC 2.0.
	isNotification := len(req.ID) == 0

	respID := req.ID
	if isNotification {
		respID = nullID
	}

	if req.JsonRpc != protocol.JsonRpcVersion {
		if !isNotification {
			writeErrorObj(stdout, respID, protocol.NewError(
				"PROTO_INVALID_REQUEST", `jsonrpc must be "2.0"`, nil))
		}
		return 0
	}

	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		if !isNotification {
			writeErrorObj(stdout, respID, protocol.NewError(
				"STORE_CONFIG_INVALID", cfgErr.Error(),
				map[string]any{"path": config.DefaultPath(), "reason": cfgErr.Error()}))
		}
		return 0
	}

	paths, pathErr := store.Resolve(cfg)
	if pathErr != nil {
		if !isNotification {
			writeErrorObj(stdout, respID, protocol.NewError(
				"INTERNAL", pathErr.Error(), nil))
		}
		return 0
	}

	var (
		result any
		rpcErr *protocol.RpcError
	)
	switch req.Method {
	case "actions/list":
		result, rpcErr = handleActionsList(paths)
	case "scripts/run":
		result, rpcErr = handleScriptsRun(context.Background(), cfg, paths, req.Params)
	default:
		rpcErr = protocol.NewError("PROTO_METHOD_NOT_FOUND",
			"unknown method: "+req.Method,
			map[string]any{"method": req.Method})
	}

	if isNotification {
		return 0
	}

	if rpcErr != nil {
		writeErrorObj(stdout, respID, rpcErr)
	} else {
		writeOK(stdout, respID, result)
	}
	return 0
}

func writeOK(w io.Writer, id json.RawMessage, result any) {
	raw, err := json.Marshal(result)
	if err != nil {
		writeErrorObj(w, id, protocol.NewError("INTERNAL", err.Error(), nil))
		return
	}
	resp := protocol.Response{
		JsonRpc: protocol.JsonRpcVersion,
		ID:      id,
		Result:  raw,
	}
	writeResponse(w, resp)
}

func writeErrorObj(w io.Writer, id json.RawMessage, e *protocol.RpcError) {
	resp := protocol.Response{
		JsonRpc: protocol.JsonRpcVersion,
		ID:      id,
		Error:   e,
	}
	writeResponse(w, resp)
}

func writeResponse(w io.Writer, resp protocol.Response) {
	body, err := json.Marshal(resp)
	if err != nil {
		// Last-resort fallback: emit a minimal internal error envelope.
		body = []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"failed to marshal response","data":{"fumiCode":"INTERNAL"}}}`)
	}
	if len(body) > protocol.MaxMessageBytes {
		body, _ = json.Marshal(protocol.Response{
			JsonRpc: protocol.JsonRpcVersion,
			ID:      resp.ID,
			Error: protocol.NewError("INTERNAL", "response exceeds 1 MiB",
				map[string]any{"sizeBytes": len(body), "limitBytes": protocol.MaxMessageBytes}),
		})
	}
	_ = protocol.WriteMessage(w, body)
}
