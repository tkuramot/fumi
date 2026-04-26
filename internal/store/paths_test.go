package store

import (
	"path/filepath"
	"testing"
)

func TestResolveUsesEnvOverride(t *testing.T) {
	t.Setenv("FUMI_STORE", "/tmp/fumi-test-store")
	p, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if p.Root != "/tmp/fumi-test-store" {
		t.Errorf("root = %q", p.Root)
	}
	if p.Actions != filepath.Join(p.Root, "actions") {
		t.Errorf("actions = %q", p.Actions)
	}
	if p.Scripts != filepath.Join(p.Root, "scripts") {
		t.Errorf("scripts = %q", p.Scripts)
	}
}

func TestResolveExpandsTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FUMI_STORE", "~/store")
	p, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "store")
	if p.Root != want {
		t.Errorf("root = %q, want %q", p.Root, want)
	}
}

func TestResolveDefaultRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FUMI_STORE", "")
	p, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "fumi")
	if p.Root != want {
		t.Errorf("root = %q, want %q", p.Root, want)
	}
}

func TestExpandTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no tilde", "/etc/passwd", "/etc/passwd"},
		{"bare tilde", "~", home},
		{"tilde slash", "~/foo/bar", filepath.Join(home, "foo", "bar")},
		{"non-home tilde untouched", "~root/x", "~root/x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandTilde(tt.in); got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}
