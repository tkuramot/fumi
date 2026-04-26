package store

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func newActionsPaths(t *testing.T) *Paths {
	t.Helper()
	root := t.TempDir()
	actions := filepath.Join(root, "actions")
	if err := os.MkdirAll(actions, 0o700); err != nil {
		t.Fatal(err)
	}
	return &Paths{Root: root, Actions: actions, Scripts: filepath.Join(root, "scripts")}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAllMissingDir(t *testing.T) {
	p := &Paths{Actions: filepath.Join(t.TempDir(), "no-such")}
	_, _, err := LoadAll(p)
	if err == nil {
		t.Fatal("expected error for missing actions dir")
	}
}

func TestLoadAll(t *testing.T) {
	p := newActionsPaths(t)

	writeFile(t, filepath.Join(p.Actions, "alpha.js"),
		"// ==Fumi Action==\n// @id alpha\n// @match https://a/*\n// ==/Fumi Action==\n")
	writeFile(t, filepath.Join(p.Actions, "beta.js"),
		"// ==Fumi Action==\n// @match https://b/*\n// ==/Fumi Action==\n") // no @id, derives "beta"
	// Duplicate id with alpha.
	writeFile(t, filepath.Join(p.Actions, "dup.js"),
		"// ==Fumi Action==\n// @id alpha\n// ==/Fumi Action==\n")
	// Malformed frontmatter (unknown key).
	writeFile(t, filepath.Join(p.Actions, "bad.js"),
		"// ==Fumi Action==\n// @runat start\n// ==/Fumi Action==\n")
	// Non-.js ignored.
	writeFile(t, filepath.Join(p.Actions, "README.md"), "ignored")
	// Subdir ignored.
	if err := os.MkdirAll(filepath.Join(p.Actions, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(p.Actions, "nested", "deep.js"), "// ==Fumi Action==\n// ==/Fumi Action==\n")

	actions, perFile, err := LoadAll(p)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	ids := make([]string, 0, len(actions))
	for _, a := range actions {
		ids = append(ids, a.ID)
	}
	sort.Strings(ids)
	wantIDs := []string{"alpha", "beta"}
	if strings.Join(ids, ",") != strings.Join(wantIDs, ",") {
		t.Errorf("ids = %v, want %v", ids, wantIDs)
	}

	// alpha must carry its full source as Code.
	for _, a := range actions {
		if a.ID == "alpha" && !strings.Contains(a.Code, "@id alpha") {
			t.Errorf("alpha.Code missing source: %q", a.Code)
		}
	}

	if len(perFile) != 2 {
		t.Fatalf("perFile = %d, want 2 (dup + bad), got %+v", len(perFile), perFile)
	}
	gotPaths := []string{perFile[0].Path, perFile[1].Path}
	sort.Strings(gotPaths)
	wantPaths := []string{"bad.js", "dup.js"}
	if strings.Join(gotPaths, ",") != strings.Join(wantPaths, ",") {
		t.Errorf("perFile paths = %v, want %v", gotPaths, wantPaths)
	}
}

func TestLoadAllEmpty(t *testing.T) {
	p := newActionsPaths(t)
	actions, perFile, err := LoadAll(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 || len(perFile) != 0 {
		t.Errorf("expected empty, got %d actions / %d perFile", len(actions), len(perFile))
	}
}
