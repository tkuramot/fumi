package store

import (
	"os"
	"path/filepath"
	"testing"
)

// makeScriptFile creates an executable file in the scripts/ directory of the given store.
func makeScriptFile(t *testing.T, p *Paths, name string) string {
	t.Helper()
	path := filepath.Join(p.Scripts, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho hi\n"), 0755); err != nil {
		t.Fatalf("create script %s: %v", name, err)
	}
	return path
}

func TestResolveScript_valid(t *testing.T) {
	p := makeTestStore(t)
	makeScriptFile(t, p, "hello.sh")

	rs, rpcErr := ResolveScript(p, "hello.sh")
	if rpcErr != nil {
		t.Fatalf("unexpected RPC error: %v", rpcErr)
	}
	if rs == nil {
		t.Fatal("expected non-nil ResolvedScript")
	}
	if rs.Cwd != p.Scripts {
		t.Errorf("Cwd = %q, want %q", rs.Cwd, p.Scripts)
	}
}

func TestResolveScript_emptyPath(t *testing.T) {
	p := makeTestStore(t)
	_, rpcErr := ResolveScript(p, "")
	if rpcErr == nil {
		t.Error("expected RPC error for empty path")
	}
	if rpcErr.Data["fumiCode"] != "SCRIPT_INVALID_PATH" {
		t.Errorf("fumiCode = %v, want SCRIPT_INVALID_PATH", rpcErr.Data["fumiCode"])
	}
}

func TestResolveScript_absolutePath(t *testing.T) {
	p := makeTestStore(t)
	_, rpcErr := ResolveScript(p, "/etc/passwd")
	if rpcErr == nil {
		t.Error("expected RPC error for absolute path")
	}
	if rpcErr.Data["fumiCode"] != "SCRIPT_INVALID_PATH" {
		t.Errorf("fumiCode = %v, want SCRIPT_INVALID_PATH", rpcErr.Data["fumiCode"])
	}
}

func TestResolveScript_parentTraversal(t *testing.T) {
	p := makeTestStore(t)
	cases := []string{
		"../escape.sh",
		"sub/../../escape.sh",
	}
	for _, rel := range cases {
		_, rpcErr := ResolveScript(p, rel)
		if rpcErr == nil {
			t.Errorf("expected RPC error for path %q", rel)
			continue
		}
		if rpcErr.Data["fumiCode"] != "SCRIPT_INVALID_PATH" {
			t.Errorf("path %q: fumiCode = %v, want SCRIPT_INVALID_PATH", rel, rpcErr.Data["fumiCode"])
		}
	}
}

func TestResolveScript_notFound(t *testing.T) {
	p := makeTestStore(t)
	_, rpcErr := ResolveScript(p, "nonexistent.sh")
	if rpcErr == nil {
		t.Error("expected RPC error for non-existent script")
	}
	if rpcErr.Data["fumiCode"] != "SCRIPT_NOT_FOUND" {
		t.Errorf("fumiCode = %v, want SCRIPT_NOT_FOUND", rpcErr.Data["fumiCode"])
	}
}

func TestResolveScript_symlink(t *testing.T) {
	p := makeTestStore(t)
	target := makeScriptFile(t, p, "real.sh")
	link := filepath.Join(p.Scripts, "link.sh")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, rpcErr := ResolveScript(p, "link.sh")
	if rpcErr == nil {
		t.Error("expected RPC error for symlink")
	}
	if rpcErr.Data["fumiCode"] != "SCRIPT_NOT_REGULAR_FILE" {
		t.Errorf("fumiCode = %v, want SCRIPT_NOT_REGULAR_FILE", rpcErr.Data["fumiCode"])
	}
}

func TestResolveScript_notExecutable(t *testing.T) {
	p := makeTestStore(t)
	path := filepath.Join(p.Scripts, "no-exec.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0600); err != nil {
		t.Fatalf("create script: %v", err)
	}

	_, rpcErr := ResolveScript(p, "no-exec.sh")
	if rpcErr == nil {
		t.Error("expected RPC error for non-executable script")
	}
	if rpcErr.Data["fumiCode"] != "SCRIPT_NOT_EXECUTABLE" {
		t.Errorf("fumiCode = %v, want SCRIPT_NOT_EXECUTABLE", rpcErr.Data["fumiCode"])
	}
}

func TestResolveScript_directory(t *testing.T) {
	p := makeTestStore(t)
	// Create a directory named like a script.
	subdir := filepath.Join(p.Scripts, "adir")
	if err := os.MkdirAll(subdir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, rpcErr := ResolveScript(p, "adir")
	if rpcErr == nil {
		t.Error("expected RPC error for directory")
	}
	if rpcErr.Data["fumiCode"] != "SCRIPT_NOT_REGULAR_FILE" {
		t.Errorf("fumiCode = %v, want SCRIPT_NOT_REGULAR_FILE", rpcErr.Data["fumiCode"])
	}
}

func TestIsWithin_child(t *testing.T) {
	if !isWithin("/a/b/c", "/a/b") {
		t.Error("expected /a/b/c to be within /a/b")
	}
}

func TestIsWithin_same(t *testing.T) {
	if !isWithin("/a/b", "/a/b") {
		t.Error("expected /a/b to be within /a/b (same path)")
	}
}

func TestIsWithin_parent(t *testing.T) {
	if isWithin("/a", "/a/b") {
		t.Error("expected /a to NOT be within /a/b")
	}
}

func TestIsWithin_sibling(t *testing.T) {
	if isWithin("/a/c", "/a/b") {
		t.Error("expected /a/c to NOT be within /a/b")
	}
}

func TestIsWithin_escape(t *testing.T) {
	if isWithin("/a/b/../../etc/passwd", "/a/b") {
		t.Error("expected path traversal to NOT be within /a/b")
	}
}
