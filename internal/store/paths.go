package store

import (
	"os"
	"path/filepath"
	"strings"
)

type Paths struct {
	Root    string
	Actions string
	Scripts string
}

func defaultRoot() string {
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".config", "fumi")
	}
	return ""
}

func expandTilde(p string) string {
	if p == "" || !strings.HasPrefix(p, "~") {
		return p
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return h
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(h, p[2:])
	}
	return p
}

// Resolve determines the store root. Priority: $FUMI_STORE > default.
func Resolve() (*Paths, error) {
	root := os.Getenv("FUMI_STORE")
	if root == "" {
		root = defaultRoot()
	}
	root = expandTilde(root)
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Paths{
		Root:    abs,
		Actions: filepath.Join(abs, "actions"),
		Scripts: filepath.Join(abs, "scripts"),
	}, nil
}
