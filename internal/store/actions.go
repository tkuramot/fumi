package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tkuramot/fumi/internal/protocol"
)

// ActionLoadError describes a single-file failure during action loading.
// The CLI surfaces these as [ERR] rows without aborting the entire listing.
type ActionLoadError struct {
	Path   string
	Reason string
}

// LoadAll enumerates every .js file directly under actions/, parses its
// frontmatter, and returns the resulting actions. File-level errors are
// returned via perFile without stopping enumeration. A non-nil err is only
// returned for a failure that prevents enumeration (e.g., missing actions/).
func LoadAll(p *Paths) (actions []protocol.Action, perFile []ActionLoadError, err error) {
	entries, err := os.ReadDir(p.Actions)
	if err != nil {
		return nil, nil, err
	}

	seen := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		full := filepath.Join(p.Actions, e.Name())
		src, readErr := os.ReadFile(full)
		if readErr != nil {
			perFile = append(perFile, ActionLoadError{Path: e.Name(), Reason: readErr.Error()})
			continue
		}
		fm, parseErr := ParseFrontmatter(string(src))
		if parseErr != nil {
			perFile = append(perFile, ActionLoadError{Path: e.Name(), Reason: parseErr.Error()})
			continue
		}
		id := fm.ID
		if id == "" {
			id = deriveIDFromFilename(e.Name())
		}
		if prev, ok := seen[id]; ok {
			perFile = append(perFile, ActionLoadError{
				Path:   e.Name(),
				Reason: fmt.Sprintf("duplicate @id %q (also in %s)", id, prev),
			})
			continue
		}
		seen[id] = e.Name()
		actions = append(actions, protocol.Action{
			ID:       id,
			Path:     e.Name(),
			Matches:  fm.Matches,
			Excludes: fm.Excludes,
			Code:     string(src),
		})
	}
	return actions, perFile, nil
}
