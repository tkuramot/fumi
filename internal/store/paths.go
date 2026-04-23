package store

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tkuramot/fumi/internal/config"
)

type Paths struct {
	Root    string
	Actions string
	Scripts string
}

func defaultRoot() string {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "fumi")
	}
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

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Resolve determines the store root. Priority: $FUMI_STORE > config.store_root > default.
func Resolve(cfg *config.Config) (*Paths, error) {
	var cfgRoot string
	if cfg != nil {
		cfgRoot = cfg.StoreRoot
	}
	root := firstNonEmpty(os.Getenv("FUMI_STORE"), cfgRoot, defaultRoot())
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
