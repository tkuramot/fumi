package store

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tkuramot/fumi/internal/protocol"
)

type ResolvedScript struct {
	AbsPath string
	Cwd     string
}

func errRpc(fumiCode, msg string, extra map[string]any) *protocol.RpcError {
	return protocol.NewError(fumiCode, msg, extra)
}

func ResolveScript(p *Paths, rel string) (*ResolvedScript, *protocol.RpcError) {
	if rel == "" || filepath.IsAbs(rel) {
		return nil, errRpc("SCRIPT_INVALID_PATH", "must be a non-empty relative path", map[string]any{"scriptPath": rel})
	}
	cleaned := filepath.Clean(rel)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return nil, errRpc("SCRIPT_INVALID_PATH", "parent traversal not allowed", map[string]any{"scriptPath": rel})
	}

	candidate := filepath.Join(p.Scripts, cleaned)

	li, err := os.Lstat(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errRpc("SCRIPT_NOT_FOUND", err.Error(), map[string]any{"scriptPath": rel})
		}
		return nil, errRpc("INTERNAL", err.Error(), nil)
	}
	if li.Mode()&os.ModeSymlink != 0 {
		return nil, errRpc("SCRIPT_NOT_REGULAR_FILE", "symlinks are rejected", map[string]any{"scriptPath": rel})
	}
	if !li.Mode().IsRegular() {
		return nil, errRpc("SCRIPT_NOT_REGULAR_FILE", "not a regular file", map[string]any{"scriptPath": rel})
	}

	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return nil, errRpc("INTERNAL", err.Error(), nil)
	}
	scriptsRoot, err := filepath.EvalSymlinks(p.Scripts)
	if err != nil {
		return nil, errRpc("STORE_NOT_FOUND", err.Error(), map[string]any{"path": p.Scripts})
	}
	if !isWithin(resolved, scriptsRoot) {
		return nil, errRpc("SCRIPT_INVALID_PATH", "resolved outside scripts/", map[string]any{"scriptPath": rel, "resolved": resolved})
	}

	if li.Mode().Perm()&0o111 == 0 {
		return nil, errRpc("SCRIPT_NOT_EXECUTABLE", "missing +x", map[string]any{"scriptPath": rel, "resolved": resolved})
	}

	return &ResolvedScript{AbsPath: resolved, Cwd: filepath.Dir(resolved)}, nil
}

func isWithin(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}
