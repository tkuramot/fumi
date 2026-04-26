package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tkuramot/fumi/internal/protocol"
)

// frame encodes body as a native messaging frame (4-byte LE length + body).
func frame(body []byte) []byte {
	var buf bytes.Buffer
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(body)))
	buf.Write(lenBuf[:])
	buf.Write(body)
	return buf.Bytes()
}

// parseResponse decodes the native messaging frame from out and returns the
// JSON-RPC Response. It fails the test if decoding fails.
func parseResponse(t *testing.T, out *bytes.Buffer) protocol.Response {
	t.Helper()
	resp, err := protocol.ReadMessage(out)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var r protocol.Response
	if err := json.Unmarshal(resp, &r); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return r
}

// makeStore creates a temp dir with actions/ and scripts/ sub-dirs and sets
// FUMI_STORE to point at it. Returns the paths struct and a cleanup function.
func makeStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "actions"), 0700); err != nil {
		t.Fatalf("mkdir actions: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	t.Setenv("FUMI_STORE", root)
	return root
}

// sendRequest sends req as a native messaging frame to run() and returns
// the stdout buffer.
func sendRequest(t *testing.T, req any) *bytes.Buffer {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var stdout, stderr bytes.Buffer
	run(bytes.NewReader(frame(body)), &stdout, &stderr)
	return &stdout
}

func TestDispatch_eof(t *testing.T) {
	makeStore(t)
	var stdout, stderr bytes.Buffer
	// Empty reader simulates Chrome closing the pipe without sending anything.
	code := run(bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	// Nothing should be written to stdout.
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty for EOF, got %d bytes", stdout.Len())
	}
}

func TestDispatch_invalidJSON(t *testing.T) {
	makeStore(t)
	var stdout, stderr bytes.Buffer
	run(bytes.NewReader(frame([]byte("{not valid json"))), &stdout, &stderr)
	r := parseResponse(t, &stdout)
	if r.Error == nil {
		t.Fatal("expected error response for invalid JSON")
	}
	if protocol.ErrorFumiCode(r.Error) != "PROTO_PARSE_ERROR" {
		t.Errorf("fumiCode = %q, want PROTO_PARSE_ERROR", protocol.ErrorFumiCode(r.Error))
	}
}

func TestDispatch_batchRequest(t *testing.T) {
	makeStore(t)
	var stdout, stderr bytes.Buffer
	run(bytes.NewReader(frame([]byte(`[{"jsonrpc":"2.0","id":"1","method":"actions/list"}]`))), &stdout, &stderr)
	r := parseResponse(t, &stdout)
	if r.Error == nil {
		t.Fatal("expected error for batch request")
	}
	if protocol.ErrorFumiCode(r.Error) != "PROTO_INVALID_REQUEST" {
		t.Errorf("fumiCode = %q, want PROTO_INVALID_REQUEST", protocol.ErrorFumiCode(r.Error))
	}
}

func TestDispatch_wrongJsonRpcVersion(t *testing.T) {
	makeStore(t)
	out := sendRequest(t, map[string]any{
		"jsonrpc": "1.0",
		"id":      "1",
		"method":  "actions/list",
	})
	r := parseResponse(t, out)
	if r.Error == nil {
		t.Fatal("expected error for wrong jsonrpc version")
	}
	if protocol.ErrorFumiCode(r.Error) != "PROTO_INVALID_REQUEST" {
		t.Errorf("fumiCode = %q, want PROTO_INVALID_REQUEST", protocol.ErrorFumiCode(r.Error))
	}
}

func TestDispatch_notification_noResponse(t *testing.T) {
	makeStore(t)
	// A notification has no "id" field — no response should be sent.
	var stdout, stderr bytes.Buffer
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "actions/list",
	})
	run(bytes.NewReader(frame(body)), &stdout, &stderr)
	if stdout.Len() != 0 {
		t.Errorf("notification should produce no output, got %d bytes", stdout.Len())
	}
}

func TestDispatch_unknownMethod(t *testing.T) {
	makeStore(t)
	out := sendRequest(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "no/such/method",
	})
	r := parseResponse(t, out)
	if r.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if protocol.ErrorFumiCode(r.Error) != "PROTO_METHOD_NOT_FOUND" {
		t.Errorf("fumiCode = %q, want PROTO_METHOD_NOT_FOUND", protocol.ErrorFumiCode(r.Error))
	}
}

func TestDispatch_actionsList_empty(t *testing.T) {
	makeStore(t)
	out := sendRequest(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "actions/list",
	})
	r := parseResponse(t, out)
	if r.Error != nil {
		t.Fatalf("unexpected error: %v", r.Error)
	}
	var result protocol.GetActionsResult
	if err := json.Unmarshal(r.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(result.Actions))
	}
}

func TestDispatch_actionsList_storeNotFound(t *testing.T) {
	// Point FUMI_STORE at a non-existent directory.
	t.Setenv("FUMI_STORE", filepath.Join(t.TempDir(), "no-such-store"))
	out := sendRequest(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "actions/list",
	})
	r := parseResponse(t, out)
	if r.Error == nil {
		t.Fatal("expected error when store not found")
	}
	if protocol.ErrorFumiCode(r.Error) != "STORE_NOT_FOUND" {
		t.Errorf("fumiCode = %q, want STORE_NOT_FOUND", protocol.ErrorFumiCode(r.Error))
	}
}

func TestDispatch_actionsList_withActions(t *testing.T) {
	storeRoot := makeStore(t)
	actionSrc := `// ==Fumi Action==
// @id hello
// @match https://example.com/*
// ==/Fumi Action==
console.log("hi");
`
	if err := os.WriteFile(
		filepath.Join(storeRoot, "actions", "hello.js"),
		[]byte(actionSrc), 0600,
	); err != nil {
		t.Fatalf("write action: %v", err)
	}

	out := sendRequest(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "actions/list",
	})
	r := parseResponse(t, out)
	if r.Error != nil {
		t.Fatalf("unexpected error: %v", r.Error)
	}
	var result protocol.GetActionsResult
	if err := json.Unmarshal(r.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(result.Actions))
	}
	if result.Actions[0].ID != "hello" {
		t.Errorf("action ID = %q, want hello", result.Actions[0].ID)
	}
}

func TestDispatch_scriptsRun_noParams(t *testing.T) {
	makeStore(t)
	out := sendRequest(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "scripts/run",
	})
	r := parseResponse(t, out)
	if r.Error == nil {
		t.Fatal("expected error when params are missing")
	}
	if protocol.ErrorFumiCode(r.Error) != "PROTO_INVALID_PARAMS" {
		t.Errorf("fumiCode = %q, want PROTO_INVALID_PARAMS", protocol.ErrorFumiCode(r.Error))
	}
}

func TestDispatch_scriptsRun_missingScriptPath(t *testing.T) {
	makeStore(t)
	out := sendRequest(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "scripts/run",
		"params": map[string]any{
			"scriptPath": "",
			"payload":    nil,
		},
	})
	r := parseResponse(t, out)
	if r.Error == nil {
		t.Fatal("expected error for empty scriptPath")
	}
	if protocol.ErrorFumiCode(r.Error) != "PROTO_INVALID_PARAMS" {
		t.Errorf("fumiCode = %q, want PROTO_INVALID_PARAMS", protocol.ErrorFumiCode(r.Error))
	}
}

func TestDispatch_scriptsRun_scriptNotFound(t *testing.T) {
	makeStore(t)
	out := sendRequest(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "scripts/run",
		"params": map[string]any{
			"scriptPath": "nonexistent.sh",
			"payload":    nil,
		},
	})
	r := parseResponse(t, out)
	if r.Error == nil {
		t.Fatal("expected error for non-existent script")
	}
	if protocol.ErrorFumiCode(r.Error) != "SCRIPT_NOT_FOUND" {
		t.Errorf("fumiCode = %q, want SCRIPT_NOT_FOUND", protocol.ErrorFumiCode(r.Error))
	}
}

func TestDispatch_scriptsRun_success(t *testing.T) {
	storeRoot := makeStore(t)
	scriptPath := filepath.Join(storeRoot, "scripts", "hello.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho world\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	out := sendRequest(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "scripts/run",
		"params": map[string]any{
			"scriptPath": "hello.sh",
			"payload":    nil,
		},
	})
	r := parseResponse(t, out)
	if r.Error != nil {
		t.Fatalf("unexpected error: %v", r.Error)
	}
	var result protocol.RunScriptResult
	if err := json.Unmarshal(r.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if len(result.Stdout) == 0 {
		t.Error("expected non-empty stdout")
	}
}

func TestDispatch_scriptsRun_invalidTimeoutMs(t *testing.T) {
	storeRoot := makeStore(t)
	scriptPath := filepath.Join(storeRoot, "scripts", "hello.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	timeoutMs := 0
	out := sendRequest(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "scripts/run",
		"params": map[string]any{
			"scriptPath": "hello.sh",
			"payload":    nil,
			"timeoutMs":  timeoutMs,
		},
	})
	r := parseResponse(t, out)
	if r.Error == nil {
		t.Fatal("expected error for timeoutMs=0")
	}
	if protocol.ErrorFumiCode(r.Error) != "PROTO_INVALID_PARAMS" {
		t.Errorf("fumiCode = %q, want PROTO_INVALID_PARAMS", protocol.ErrorFumiCode(r.Error))
	}
}

func TestDispatch_idReflected(t *testing.T) {
	makeStore(t)
	out := sendRequest(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      "my-unique-id",
		"method":  "no/such/method",
	})
	r := parseResponse(t, out)
	var id string
	if err := json.Unmarshal(r.ID, &id); err != nil {
		t.Fatalf("unmarshal id: %v", err)
	}
	if id != "my-unique-id" {
		t.Errorf("id = %q, want my-unique-id", id)
	}
}
