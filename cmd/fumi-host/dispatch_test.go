package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tkuramot/fumi/internal/protocol"
)

func setupStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "actions"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FUMI_STORE", root)
	t.Setenv("HOME", t.TempDir()) // ensure config.Load() returns defaults
	return root
}

func frame(t *testing.T, body string) io.Reader {
	t.Helper()
	var buf bytes.Buffer
	if err := protocol.WriteMessage(&buf, []byte(body)); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func readResp(t *testing.T, r io.Reader) protocol.Response {
	t.Helper()
	body, err := protocol.ReadMessage(r)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	var resp protocol.Response
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, body)
	}
	return resp
}

func TestDispatchEOFNoOutput(t *testing.T) {
	setupStore(t)
	var stdout, stderr bytes.Buffer
	if code := run(bytes.NewReader(nil), &stdout, &stderr); code != 0 {
		t.Errorf("exit code = %d", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %q", stdout.Bytes())
	}
}

func TestDispatchErrorCases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	tests := []struct {
		name        string
		body        string
		wantFumi    string
		notification bool
	}{
		{
			name:     "invalid JSON",
			body:     `{not json`,
			wantFumi: "PROTO_PARSE_ERROR",
		},
		{
			name:     "batch rejected",
			body:     `[{"jsonrpc":"2.0","id":1,"method":"actions/list"}]`,
			wantFumi: "PROTO_INVALID_REQUEST",
		},
		{
			name:     "wrong jsonrpc version",
			body:     `{"jsonrpc":"1.0","id":1,"method":"actions/list"}`,
			wantFumi: "PROTO_INVALID_REQUEST",
		},
		{
			name:     "unknown method",
			body:     `{"jsonrpc":"2.0","id":1,"method":"does/not/exist"}`,
			wantFumi: "PROTO_METHOD_NOT_FOUND",
		},
		{
			name:     "scripts/run missing params",
			body:     `{"jsonrpc":"2.0","id":1,"method":"scripts/run"}`,
			wantFumi: "PROTO_INVALID_PARAMS",
		},
		{
			name:     "scripts/run unknown field",
			body:     `{"jsonrpc":"2.0","id":1,"method":"scripts/run","params":{"scriptPath":"x","payload":null,"bogus":1}}`,
			wantFumi: "PROTO_INVALID_PARAMS",
		},
		{
			name:     "scripts/run missing scriptPath",
			body:     `{"jsonrpc":"2.0","id":1,"method":"scripts/run","params":{"payload":null}}`,
			wantFumi: "PROTO_INVALID_PARAMS",
		},
		{
			name:     "scripts/run script not found",
			body:     `{"jsonrpc":"2.0","id":1,"method":"scripts/run","params":{"scriptPath":"missing.sh","payload":null}}`,
			wantFumi: "SCRIPT_NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupStore(t)
			var stdout, stderr bytes.Buffer
			if code := run(frame(t, tt.body), &stdout, &stderr); code != 0 {
				t.Fatalf("exit code = %d", code)
			}
			resp := readResp(t, &stdout)
			if resp.Error == nil {
				t.Fatalf("expected error, got %+v", resp)
			}
			if got := protocol.ErrorFumiCode(resp.Error); got != tt.wantFumi {
				t.Fatalf("fumiCode = %q, want %q (msg=%q)", got, tt.wantFumi, resp.Error.Message)
			}
		})
	}
}

func TestDispatchActionsListEmpty(t *testing.T) {
	setupStore(t)
	var stdout, stderr bytes.Buffer
	body := `{"jsonrpc":"2.0","id":42,"method":"actions/list"}`
	if code := run(frame(t, body), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	resp := readResp(t, &stdout)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if string(resp.ID) != "42" {
		t.Errorf("id = %s", resp.ID)
	}
	var got protocol.GetActionsResult
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Actions) != 0 {
		t.Errorf("actions = %v, want empty", got.Actions)
	}
}

func TestDispatchActionsListMissingDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FUMI_STORE", root)
	t.Setenv("HOME", t.TempDir())

	var stdout, stderr bytes.Buffer
	body := `{"jsonrpc":"2.0","id":1,"method":"actions/list"}`
	if code := run(frame(t, body), &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	resp := readResp(t, &stdout)
	if resp.Error == nil {
		t.Fatalf("expected error")
	}
	if got := protocol.ErrorFumiCode(resp.Error); got != "STORE_NOT_FOUND" {
		t.Errorf("fumiCode = %q", got)
	}
}

func TestDispatchNotificationProducesNoResponse(t *testing.T) {
	setupStore(t)
	var stdout, stderr bytes.Buffer
	// No "id" field => notification.
	body := `{"jsonrpc":"2.0","method":"actions/list"}`
	if code := run(frame(t, body), &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("notification produced output: %q", stdout.Bytes())
	}
}

func TestDispatchScriptsRunOK(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	root := setupStore(t)
	scriptPath := filepath.Join(root, "scripts", "echo.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\ncat\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	body := `{"jsonrpc":"2.0","id":1,"method":"scripts/run","params":{"scriptPath":"echo.sh","payload":{"hello":"world"},"timeoutMs":5000}}`
	if code := run(frame(t, body), &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	resp := readResp(t, &stdout)
	if resp.Error != nil {
		t.Fatalf("err: %+v", resp.Error)
	}
	var res protocol.RunScriptResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit = %d", res.ExitCode)
	}
	if res.Stdout != `{"hello":"world"}` {
		t.Errorf("stdout = %q", res.Stdout)
	}
}
