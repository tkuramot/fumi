package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tkuramot/fumi/internal/protocol"
	"github.com/tkuramot/fumi/internal/store"
)

func resolveScript(t *testing.T, body string) (*store.Paths, *store.ResolvedScript) {
	t.Helper()
	root := t.TempDir()
	scripts := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scripts, 0o700); err != nil {
		t.Fatal(err)
	}
	p := &store.Paths{Root: root, Actions: filepath.Join(root, "actions"), Scripts: scripts}
	if err := os.WriteFile(filepath.Join(scripts, "run.sh"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	rs, rerr := store.ResolveScript(p, "run.sh")
	if rerr != nil {
		t.Fatalf("resolve: %+v", rerr)
	}
	return p, rs
}

func TestRunZeroTimeoutInvalid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	_, rs := resolveScript(t, "#!/bin/sh\nexit 0\n")
	_, rerr := Run(context.Background(), &RunParams{Script: rs, Timeout: 0})
	if rerr == nil {
		t.Fatal("expected error")
	}
	if got := protocol.ErrorFumiCode(rerr); got != "PROTO_INVALID_PARAMS" {
		t.Errorf("fumiCode = %q", got)
	}
}

func TestRunHappyPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	p, rs := resolveScript(t, "#!/bin/sh\ncat\necho err >&2\nexit 0\n")
	out, rerr := Run(context.Background(), &RunParams{
		Script:    rs,
		Payload:   json.RawMessage(`{"x":1}`),
		Timeout:   5 * time.Second,
		StoreRoot: p.Root,
	})
	if rerr != nil {
		t.Fatalf("rpc err: %+v", rerr)
	}
	if out.ExitCode != 0 {
		t.Errorf("exit = %d", out.ExitCode)
	}
	if string(out.Stdout) != `{"x":1}` {
		t.Errorf("stdout = %q", out.Stdout)
	}
	if !strings.Contains(string(out.Stderr), "err") {
		t.Errorf("stderr = %q", out.Stderr)
	}
}

func TestRunNonZeroExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	_, rs := resolveScript(t, "#!/bin/sh\nexit 7\n")
	out, rerr := Run(context.Background(), &RunParams{Script: rs, Timeout: 5 * time.Second})
	if rerr != nil {
		t.Fatalf("rpc err: %+v", rerr)
	}
	if out.ExitCode != 7 {
		t.Errorf("exit = %d, want 7", out.ExitCode)
	}
}

func TestRunNullPayloadWhenEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	_, rs := resolveScript(t, "#!/bin/sh\ncat\n")
	out, rerr := Run(context.Background(), &RunParams{Script: rs, Timeout: 5 * time.Second})
	if rerr != nil {
		t.Fatalf("%+v", rerr)
	}
	if string(out.Stdout) != "null" {
		t.Errorf("stdout = %q, want %q", out.Stdout, "null")
	}
}

func TestRunTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	_, rs := resolveScript(t, "#!/bin/sh\nsleep 5\n")
	_, rerr := Run(context.Background(), &RunParams{Script: rs, Timeout: 100 * time.Millisecond})
	if rerr == nil {
		t.Fatal("expected timeout error")
	}
	if got := protocol.ErrorFumiCode(rerr); got != "EXEC_TIMEOUT" {
		t.Errorf("fumiCode = %q", got)
	}
}

func TestRunStdoutTooLarge(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	// Emit > StdoutLimit (768 KiB) bytes.
	body := "#!/bin/sh\nexec dd if=/dev/zero bs=1024 count=1024 2>/dev/null\n"
	_, rs := resolveScript(t, body)
	_, rerr := Run(context.Background(), &RunParams{Script: rs, Timeout: 10 * time.Second})
	if rerr == nil {
		t.Fatal("expected output too large error")
	}
	if got := protocol.ErrorFumiCode(rerr); got != "EXEC_OUTPUT_TOO_LARGE" {
		t.Errorf("fumiCode = %q", got)
	}
}

func TestBuildEnvScrubsFumiAndSetsStore(t *testing.T) {
	t.Setenv("FUMI_LEAKED", "should-not-appear")
	t.Setenv("FUMI_OTHER", "x")
	t.Setenv("OTHER_VAR", "kept")

	env := buildEnv("/tmp/store-root")
	var sawStore, sawOther bool
	for _, e := range env {
		if strings.HasPrefix(e, "FUMI_LEAKED=") || strings.HasPrefix(e, "FUMI_OTHER=") {
			t.Errorf("inherited FUMI_* leaked: %q", e)
		}
		if e == "FUMI_STORE=/tmp/store-root" {
			sawStore = true
		}
		if e == "OTHER_VAR=kept" {
			sawOther = true
		}
	}
	if !sawStore {
		t.Error("FUMI_STORE not set")
	}
	if !sawOther {
		t.Error("non-FUMI env not preserved")
	}
}

func TestCappedBuffer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		limit     int
		writes    []string
		wantBytes string
		wantOver  bool
	}{
		{"under limit", 10, []string{"abc", "de"}, "abcde", false},
		{"exact limit", 5, []string{"abcde"}, "abcde", false},
		{"single write overflows", 5, []string{"abcdefg"}, "abcde", true},
		{"second write overflows", 5, []string{"abc", "defgh"}, "abcde", true},
		{"writes after overflow ignored", 3, []string{"abcd", "ef"}, "abc", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := &cappedBuffer{limit: tt.limit}
			for _, w := range tt.writes {
				n, err := b.Write([]byte(w))
				if err != nil {
					t.Fatal(err)
				}
				if n != len(w) {
					t.Errorf("Write returned %d, want %d (must always claim full length)", n, len(w))
				}
			}
			if string(b.Bytes()) != tt.wantBytes {
				t.Errorf("bytes = %q, want %q", b.Bytes(), tt.wantBytes)
			}
			if b.Overflowed() != tt.wantOver {
				t.Errorf("overflowed = %v, want %v", b.Overflowed(), tt.wantOver)
			}
		})
	}
}
