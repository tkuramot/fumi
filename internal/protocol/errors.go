package protocol

var fumiCodeToNumeric = map[string]int{
	"PROTO_PARSE_ERROR":         CodeParseError,
	"PROTO_INVALID_REQUEST":     CodeInvalidRequest,
	"PROTO_METHOD_NOT_FOUND":    CodeMethodNotFound,
	"PROTO_INVALID_PARAMS":      CodeInvalidParams,
	"INTERNAL":                  CodeInternal,
	"STORE_NOT_FOUND":           -33001,
	"STORE_CONFIG_INVALID":      -33002,
	"STORE_ACTIONS_TOO_LARGE":   -33010,
	"STORE_FRONTMATTER_INVALID": -33011,
	"SCRIPT_INVALID_PATH":       -33020,
	"SCRIPT_NOT_FOUND":          -33021,
	"SCRIPT_NOT_REGULAR_FILE":   -33022,
	"SCRIPT_NOT_EXECUTABLE":     -33023,
	"EXEC_TIMEOUT":              -33030,
	"EXEC_OUTPUT_TOO_LARGE":     -33031,
	"EXEC_SPAWN_FAILED":         -33032,
}

func NewError(fumiCode, message string, extra map[string]any) *RpcError {
	data := map[string]any{"fumiCode": fumiCode}
	for k, v := range extra {
		data[k] = v
	}
	code, ok := fumiCodeToNumeric[fumiCode]
	if !ok {
		code = CodeInternal
	}
	return &RpcError{Code: code, Message: message, Data: data}
}

func ErrorFumiCode(e *RpcError) string {
	if e == nil || e.Data == nil {
		return ""
	}
	if s, ok := e.Data["fumiCode"].(string); ok {
		return s
	}
	return ""
}
