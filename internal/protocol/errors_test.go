package protocol

import (
	"testing"
)

func TestNewError_knownCode(t *testing.T) {
	cases := []struct {
		fumiCode    string
		wantNumeric int
	}{
		{"PROTO_PARSE_ERROR", CodeParseError},
		{"PROTO_INVALID_REQUEST", CodeInvalidRequest},
		{"PROTO_METHOD_NOT_FOUND", CodeMethodNotFound},
		{"PROTO_INVALID_PARAMS", CodeInvalidParams},
		{"INTERNAL", CodeInternal},
		{"STORE_NOT_FOUND", -33001},
		{"STORE_CONFIG_INVALID", -33002},
		{"STORE_ACTIONS_TOO_LARGE", -33010},
		{"STORE_FRONTMATTER_INVALID", -33011},
		{"SCRIPT_INVALID_PATH", -33020},
		{"SCRIPT_NOT_FOUND", -33021},
		{"SCRIPT_NOT_REGULAR_FILE", -33022},
		{"SCRIPT_NOT_EXECUTABLE", -33023},
		{"EXEC_TIMEOUT", -33030},
		{"EXEC_OUTPUT_TOO_LARGE", -33031},
		{"EXEC_SPAWN_FAILED", -33032},
	}
	for _, tc := range cases {
		e := NewError(tc.fumiCode, "test message", nil)
		if e.Code != tc.wantNumeric {
			t.Errorf("NewError(%q): code = %d, want %d", tc.fumiCode, e.Code, tc.wantNumeric)
		}
		if e.Message != "test message" {
			t.Errorf("NewError(%q): message = %q, want %q", tc.fumiCode, e.Message, "test message")
		}
		if s, ok := e.Data["fumiCode"].(string); !ok || s != tc.fumiCode {
			t.Errorf("NewError(%q): data[fumiCode] = %v, want %q", tc.fumiCode, e.Data["fumiCode"], tc.fumiCode)
		}
	}
}

func TestNewError_unknownCode(t *testing.T) {
	e := NewError("TOTALLY_UNKNOWN", "oops", nil)
	if e.Code != CodeInternal {
		t.Errorf("expected CodeInternal for unknown fumiCode, got %d", e.Code)
	}
}

func TestNewError_extraFields(t *testing.T) {
	e := NewError("INTERNAL", "boom", map[string]any{
		"path":   "/tmp/foo",
		"detail": 42,
	})
	if e.Data["path"] != "/tmp/foo" {
		t.Errorf("extra field path = %v, want /tmp/foo", e.Data["path"])
	}
	if e.Data["detail"] != 42 {
		t.Errorf("extra field detail = %v, want 42", e.Data["detail"])
	}
	if e.Data["fumiCode"] != "INTERNAL" {
		t.Errorf("fumiCode = %v, want INTERNAL", e.Data["fumiCode"])
	}
}

func TestNewError_Error(t *testing.T) {
	e := NewError("INTERNAL", "something went wrong", nil)
	if e.Error() != "something went wrong" {
		t.Errorf("Error() = %q, want %q", e.Error(), "something went wrong")
	}
}

func TestErrorFumiCode_valid(t *testing.T) {
	e := NewError("EXEC_TIMEOUT", "timed out", nil)
	if got := ErrorFumiCode(e); got != "EXEC_TIMEOUT" {
		t.Errorf("ErrorFumiCode = %q, want EXEC_TIMEOUT", got)
	}
}

func TestErrorFumiCode_nil(t *testing.T) {
	if got := ErrorFumiCode(nil); got != "" {
		t.Errorf("ErrorFumiCode(nil) = %q, want empty", got)
	}
}

func TestErrorFumiCode_noData(t *testing.T) {
	e := &RpcError{Code: -32603, Message: "oops", Data: nil}
	if got := ErrorFumiCode(e); got != "" {
		t.Errorf("ErrorFumiCode with nil Data = %q, want empty", got)
	}
}

func TestErrorFumiCode_wrongType(t *testing.T) {
	e := &RpcError{
		Code:    -32603,
		Message: "oops",
		Data:    map[string]any{"fumiCode": 12345},
	}
	if got := ErrorFumiCode(e); got != "" {
		t.Errorf("ErrorFumiCode with non-string fumiCode = %q, want empty", got)
	}
}
