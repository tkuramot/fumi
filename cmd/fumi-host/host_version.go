package main

import "github.com/tkuramot/fumi/internal/protocol"

// handleHostVersion reports the host build version so the extension can detect
// a version skew. Read-only; same value as `fumi-host --version`.
func handleHostVersion() (any, *protocol.RpcError) {
	return protocol.HostVersionResult{Version: resolveVersion()}, nil
}
