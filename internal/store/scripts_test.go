package store

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tkuramot/fumi/internal/protocol"
)

func newPaths(t *testing.T) *Paths {
	t.Helper()
	root := t.TempDir()
	scripts := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scripts, 0o700); err != nil {
		t.Fatal(err)
	}
	return &Paths{Root: root, Actions: filepath.Join(root, "actions"), Scripts: scripts}
}

func writeExec(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestResolveScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix permissions only")
	}

	p := newPaths(t)
	writeExec(t, filepath.Join(p.Scripts, "ok.sh"), "#!/bin/sh\necho hi\n")
	writeExec(t, filepath.Join(p.Scripts, "sub", "nested.sh"), "#!/bin/sh\n")

	notExec := filepath.Join(p.Scripts, "no-x.sh")
	if err := os.WriteFile(notExec, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink("ok.sh", filepath.Join(p.Scripts, "link.sh")); err != nil {
		t.Fatal(err)
	}

	dirPath := filepath.Join(p.Scripts, "dir")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		rel      string
		wantCode string // empty = success
	}{
		{"empty path", "", "SCRIPT_INVALID_PATH"},
		{"absolute path", "/etc/passwd", "SCRIPT_INVALID_PATH"},
		{"parent traversal", "../etc/passwd", "SCRIPT_INVALID_PATH"},
		{"embedded traversal", "sub/../../etc/passwd", "SCRIPT_INVALID_PATH"},
		{"missing file", "missing.sh", "SCRIPT_NOT_FOUND"},
		{"directory", "dir", "SCRIPT_NOT_REGULAR_FILE"},
		{"symlink", "link.sh", "SCRIPT_NOT_REGULAR_FILE"},
		{"not executable", "no-x.sh", "SCRIPT_NOT_EXECUTABLE"},
		{"happy path", "ok.sh", ""},
		{"happy nested", "sub/nested.sh", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs, rerr := ResolveScript(p, tt.rel)
			if tt.wantCode == "" {
				if rerr != nil {
					t.Fatalf("unexpected err: %+v", rerr)
				}
				if rs == nil || rs.AbsPath == "" {
					t.Fatalf("nil or empty resolved")
				}
				if rs.Cwd != filepath.Dir(rs.AbsPath) {
					t.Errorf("cwd = %q, want %q", rs.Cwd, filepath.Dir(rs.AbsPath))
				}
				return
			}
			if rerr == nil {
				t.Fatalf("expected error %s, got success", tt.wantCode)
			}
			if got := protocol.ErrorFumiCode(rerr); got != tt.wantCode {
				t.Fatalf("fumiCode = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

func TestIsWithin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		child, parent  string
		want           bool
	}{
		{"same dir", "/a/b", "/a/b", true},
		{"child", "/a/b/c", "/a/b", true},
		{"sibling", "/a/c", "/a/b", false},
		{"parent of", "/a", "/a/b", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isWithin(tt.child, tt.parent); got != tt.want {
				t.Errorf("got %v want %v", got, tt.want)
			}
		})
	}
}
