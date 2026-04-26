package store

import (
	"os"
	"path/filepath"
	"testing"
)

// makeTestStore creates a temp directory with actions/ and scripts/ sub-dirs
// and returns a *Paths pointing to it.
func makeTestStore(t *testing.T) *Paths {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "actions"), 0700); err != nil {
		t.Fatalf("mkdir actions: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	abs, _ := filepath.Abs(root)
	return &Paths{
		Root:    abs,
		Actions: filepath.Join(abs, "actions"),
		Scripts: filepath.Join(abs, "scripts"),
	}
}

// writeAction writes a .js file under actions/ with the given content.
func writeAction(t *testing.T, p *Paths, name, content string) {
	t.Helper()
	path := filepath.Join(p.Actions, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write action %s: %v", name, err)
	}
}

func TestLoadAll_empty(t *testing.T) {
	p := makeTestStore(t)
	actions, perFile, err := LoadAll(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
	if len(perFile) != 0 {
		t.Errorf("expected 0 perFile errors, got %d", len(perFile))
	}
}

func TestLoadAll_singleActionWithFrontmatter(t *testing.T) {
	p := makeTestStore(t)
	writeAction(t, p, "save-note.js", `// ==Fumi Action==
// @id save-note
// @match https://example.com/*
// ==/Fumi Action==
console.log("save");
`)
	actions, perFile, err := LoadAll(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(perFile) != 0 {
		t.Errorf("unexpected perFile errors: %v", perFile)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.ID != "save-note" {
		t.Errorf("ID = %q, want save-note", a.ID)
	}
	if len(a.Matches) != 1 || a.Matches[0] != "https://example.com/*" {
		t.Errorf("Matches = %v, want [https://example.com/*]", a.Matches)
	}
	if a.Path != "save-note.js" {
		t.Errorf("Path = %q, want save-note.js", a.Path)
	}
}

func TestLoadAll_derivedIDFromFilename(t *testing.T) {
	p := makeTestStore(t)
	writeAction(t, p, "my action.js", `// ==Fumi Action==
// @match https://example.com/*
// ==/Fumi Action==
`)
	actions, perFile, err := LoadAll(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(perFile) != 0 {
		t.Errorf("unexpected perFile errors: %v", perFile)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].ID != "my-action" {
		t.Errorf("derived ID = %q, want my-action", actions[0].ID)
	}
}

func TestLoadAll_skipsNonJSFiles(t *testing.T) {
	p := makeTestStore(t)
	if err := os.WriteFile(filepath.Join(p.Actions, "readme.txt"), []byte("hello"), 0600); err != nil {
		t.Fatalf("write txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(p.Actions, "script.sh"), []byte("#!/bin/sh"), 0600); err != nil {
		t.Fatalf("write sh: %v", err)
	}
	writeAction(t, p, "valid.js", `// ==Fumi Action==
// @id valid
// @match https://example.com/*
// ==/Fumi Action==
`)
	actions, perFile, err := LoadAll(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(perFile) != 0 {
		t.Errorf("unexpected perFile errors: %v", perFile)
	}
	if len(actions) != 1 {
		t.Errorf("expected 1 action (only .js), got %d", len(actions))
	}
}

func TestLoadAll_skipsSubdirectories(t *testing.T) {
	p := makeTestStore(t)
	// Create a subdirectory in actions/
	subdir := filepath.Join(p.Actions, "subdir")
	if err := os.MkdirAll(subdir, 0700); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	// Even a .js named dir won't be read (IsDir check)
	actions, _, err := LoadAll(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions (directories skipped), got %d", len(actions))
	}
}

func TestLoadAll_duplicateID(t *testing.T) {
	p := makeTestStore(t)
	// Both files resolve to the same ID "my-action".
	writeAction(t, p, "a-my-action.js", `// ==Fumi Action==
// @id my-action
// @match https://example.com/*
// ==/Fumi Action==
`)
	writeAction(t, p, "b-my-action.js", `// ==Fumi Action==
// @id my-action
// @match https://other.com/*
// ==/Fumi Action==
`)
	actions, perFile, err := LoadAll(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// One action loaded, one rejected as duplicate.
	if len(actions) != 1 {
		t.Errorf("expected 1 action (duplicate rejected), got %d", len(actions))
	}
	if len(perFile) != 1 {
		t.Errorf("expected 1 perFile error (duplicate), got %d", len(perFile))
	}
}

func TestLoadAll_parseError(t *testing.T) {
	p := makeTestStore(t)
	writeAction(t, p, "broken.js", `// ==Fumi Action==
// @unknown key
// ==/Fumi Action==
`)
	actions, perFile, err := LoadAll(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
	if len(perFile) != 1 {
		t.Errorf("expected 1 perFile error, got %d", len(perFile))
	}
}

func TestLoadAll_actionsDirectoryNotFound(t *testing.T) {
	root := t.TempDir()
	abs, _ := filepath.Abs(root)
	p := &Paths{
		Root:    abs,
		Actions: filepath.Join(abs, "actions"), // does not exist
		Scripts: filepath.Join(abs, "scripts"),
	}
	_, _, err := LoadAll(p)
	if err == nil {
		t.Error("expected error when actions directory does not exist")
	}
}

func TestLoadAll_codePreserved(t *testing.T) {
	p := makeTestStore(t)
	code := `// ==Fumi Action==
// @id coder
// @match https://example.com/*
// ==/Fumi Action==
const x = 42;
console.log(x);
`
	writeAction(t, p, "coder.js", code)
	actions, _, err := LoadAll(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Code != code {
		t.Errorf("Code mismatch:\ngot:  %q\nwant: %q", actions[0].Code, code)
	}
}
