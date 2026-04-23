package main

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/tkuramot/fumi/internal/config"
	"github.com/tkuramot/fumi/internal/protocol"
	"github.com/tkuramot/fumi/internal/runner"
	"github.com/tkuramot/fumi/internal/store"
)

func handleScriptsRun(ctx context.Context, cfg *config.Config, paths *store.Paths, raw json.RawMessage) (any, *protocol.RpcError) {
	if len(raw) == 0 {
		return nil, protocol.NewError("PROTO_INVALID_PARAMS", "params are required", nil)
	}

	var params protocol.RunScriptParams
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&params); err != nil {
		return nil, protocol.NewError("PROTO_INVALID_PARAMS", err.Error(), nil)
	}
	if params.ScriptPath == "" {
		return nil, protocol.NewError("PROTO_INVALID_PARAMS", "scriptPath is required", nil)
	}
	if len(params.Payload) == 0 {
		return nil, protocol.NewError("PROTO_INVALID_PARAMS", "payload is required (use null if unused)", nil)
	}

	timeout := cfg.DefaultTimeout()
	if params.TimeoutMs != nil {
		if *params.TimeoutMs <= 0 {
			return nil, protocol.NewError("PROTO_INVALID_PARAMS",
				"timeoutMs must be > 0", map[string]any{"timeoutMs": *params.TimeoutMs})
		}
		timeout = time.Duration(*params.TimeoutMs) * time.Millisecond
	}

	resolved, rpcErr := store.ResolveScript(paths, params.ScriptPath)
	if rpcErr != nil {
		return nil, rpcErr
	}

	outcome, rpcErr := runner.Run(ctx, &runner.RunParams{
		Script:    resolved,
		Payload:   params.Payload,
		Timeout:   timeout,
		StoreRoot: paths.Root,
		ExtraEnv:  params.Context,
	})
	if rpcErr != nil {
		return nil, rpcErr
	}

	return protocol.RunScriptResult{
		ExitCode:   outcome.ExitCode,
		Stdout:     string(outcome.Stdout),
		Stderr:     string(outcome.Stderr),
		DurationMs: outcome.DurationMs,
	}, nil
}
