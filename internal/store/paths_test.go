package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandTilde_empty(t *testing.T) {
	got := expandTilde("")
	if got != "" {
		t.Errorf("expandTilde(\"\") = %q, want empty", got)
	}
}

func TestExpandTilde_noTilde(t *testing.T) {
	got := expandTilde("/absolute/path")
	if got != "/absolute/path" {
		t.Errorf("expandTilde(\"/absolute/path\") = %q, want same", got)
	}
}

func TestExpandTilde_tilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir:", err)
	}
	got := expandTilde("~")
	if got != home {
		t.Errorf("expandTilde(\"~\") = %q, want %q", got, home)
	}
}

func TestExpandTilde_tildeSlash(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir:", err)
	}
	got := expandTilde("~/foo/bar")
	want := filepath.Join(home, "foo", "bar")
	if got != want {
		t.Errorf("expandTilde(\"~/foo/bar\") = %q, want %q", got, want)
	}
}

func TestExpandTilde_tildeOtherUser(t *testing.T) {
	// ~other is not expanded; returned as-is.
	got := expandTilde("~otheruser/path")
	if got != "~otheruser/path" {
		t.Errorf("expandTilde(\"~otheruser/path\") = %q, want unchanged", got)
	}
}

func TestResolve_defaultRoot(t *testing.T) {
	t.Setenv("FUMI_STORE", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir:", err)
	}
	p, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	wantRoot := filepath.Join(home, ".config", "fumi")
	if p.Root != wantRoot {
		t.Errorf("Root = %q, want %q", p.Root, wantRoot)
	}
	if p.Actions != filepath.Join(wantRoot, "actions") {
		t.Errorf("Actions = %q", p.Actions)
	}
	if p.Scripts != filepath.Join(wantRoot, "scripts") {
		t.Errorf("Scripts = %q", p.Scripts)
	}
}

func TestResolve_fumiStoreEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FUMI_STORE", dir)

	p, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	// Resolve returns absolute path.
	abs, _ := filepath.Abs(dir)
	if p.Root != abs {
		t.Errorf("Root = %q, want %q", p.Root, abs)
	}
	if p.Actions != filepath.Join(abs, "actions") {
		t.Errorf("Actions = %q", p.Actions)
	}
}

func TestResolve_fumiStoreTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir:", err)
	}
	t.Setenv("FUMI_STORE", "~/mystore")

	p, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	want := filepath.Join(home, "mystore")
	if !strings.HasPrefix(p.Root, want) {
		t.Errorf("Root = %q, want prefix %q", p.Root, want)
	}
}
