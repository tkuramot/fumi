package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"strings"

	"github.com/tkuramot/fumi/internal/protocol"
	"github.com/tkuramot/fumi/internal/store"
)

func handleActionsList(paths *store.Paths) (any, *protocol.RpcError) {
	actions, perFile, err := store.LoadAll(paths)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, protocol.NewError("STORE_NOT_FOUND",
				"actions directory not found",
				map[string]any{"path": paths.Actions})
		}
		return nil, protocol.NewError("INTERNAL", err.Error(), nil)
	}

	if len(perFile) > 0 {
		first := perFile[0]
		reasons := make([]string, 0, len(perFile))
		for _, pf := range perFile {
			reasons = append(reasons, pf.Path+": "+pf.Reason)
		}
		return nil, protocol.NewError("STORE_FRONTMATTER_INVALID",
			first.Path+": "+first.Reason,
			map[string]any{
				"path":   first.Path,
				"reason": strings.Join(reasons, "; "),
			})
	}

	if actions == nil {
		actions = []protocol.Action{}
	}
	result := protocol.GetActionsResult{Actions: actions}

	// Pre-check serialized size against the 1 MiB wire limit so we can map to
	// STORE_ACTIONS_TOO_LARGE instead of a generic transport failure.
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, protocol.NewError("INTERNAL", err.Error(), nil)
	}
	if len(encoded) > protocol.MaxMessageBytes {
		return nil, protocol.NewError("STORE_ACTIONS_TOO_LARGE",
			"actions/list response exceeds 1 MiB",
			map[string]any{
				"sizeBytes":  len(encoded),
				"limitBytes": protocol.MaxMessageBytes,
			})
	}
	return result, nil
}
