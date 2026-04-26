package protocol

import "testing"

func TestNewError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fumiCode string
		wantCode int
	}{
		{"parse error", "PROTO_PARSE_ERROR", CodeParseError},
		{"invalid request", "PROTO_INVALID_REQUEST", CodeInvalidRequest},
		{"method not found", "PROTO_METHOD_NOT_FOUND", CodeMethodNotFound},
		{"invalid params", "PROTO_INVALID_PARAMS", CodeInvalidParams},
		{"internal", "INTERNAL", CodeInternal},
		{"store not found", "STORE_NOT_FOUND", -33001},
		{"script invalid path", "SCRIPT_INVALID_PATH", -33020},
		{"exec timeout", "EXEC_TIMEOUT", -33030},
		{"unknown maps to internal", "NOT_A_REAL_CODE", CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := NewError(tt.fumiCode, "msg", nil)
			if e.Code != tt.wantCode {
				t.Errorf("code = %d, want %d", e.Code, tt.wantCode)
			}
			if e.Message != "msg" {
				t.Errorf("message = %q", e.Message)
			}
			if got := ErrorFumiCode(e); got != tt.fumiCode {
				t.Errorf("fumiCode = %q, want %q", got, tt.fumiCode)
			}
		})
	}
}

func TestNewErrorPreservesExtra(t *testing.T) {
	t.Parallel()
	e := NewError("STORE_NOT_FOUND", "missing", map[string]any{"path": "/tmp/x", "n": 5})
	if got, _ := e.Data["path"].(string); got != "/tmp/x" {
		t.Errorf("path = %v", e.Data["path"])
	}
	if got, _ := e.Data["n"].(int); got != 5 {
		t.Errorf("n = %v", e.Data["n"])
	}
	if got, _ := e.Data["fumiCode"].(string); got != "STORE_NOT_FOUND" {
		t.Errorf("fumiCode = %v", e.Data["fumiCode"])
	}
}

func TestErrorFumiCodeNil(t *testing.T) {
	t.Parallel()
	if got := ErrorFumiCode(nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
	if got := ErrorFumiCode(&RpcError{}); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestRpcErrorImplementsError(t *testing.T) {
	t.Parallel()
	var err error = &RpcError{Message: "boom"}
	if err.Error() != "boom" {
		t.Fatalf("Error() = %q", err.Error())
	}
}
