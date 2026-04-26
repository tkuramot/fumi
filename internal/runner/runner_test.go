package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tkuramot/fumi/internal/store"
)

// makeScript creates a temporary executable shell script and returns a
// *store.ResolvedScript pointing to it.
func makeScript(t *testing.T, content string) *store.ResolvedScript {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "script.sh")
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return &store.ResolvedScript{AbsPath: path, Cwd: dir}
}

// ---- cappedBuffer tests ----

func TestCappedBuffer_withinLimit(t *testing.T) {
	b := &cappedBuffer{limit: 10}
	n, err := b.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("n = %d, want 5", n)
	}
	if b.Overflowed() {
		t.Error("expected Overflowed()=false")
	}
	if string(b.Bytes()) != "hello" {
		t.Errorf("bytes = %q, want hello", b.Bytes())
	}
}

func TestCappedBuffer_exactLimit(t *testing.T) {
	b := &cappedBuffer{limit: 5}
	b.Write([]byte("hello"))
	if b.Overflowed() {
		t.Error("expected Overflowed()=false for exact limit")
	}
	if string(b.Bytes()) != "hello" {
		t.Errorf("bytes = %q, want hello", b.Bytes())
	}
}

func TestCappedBuffer_overflow(t *testing.T) {
	b := &cappedBuffer{limit: 4}
	n, err := b.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Write reports the full length even when capped.
	if n != 5 {
		t.Errorf("n = %d, want 5 (full input length)", n)
	}
	if !b.Overflowed() {
		t.Error("expected Overflowed()=true")
	}
	// Only the first 4 bytes should be retained.
	if string(b.Bytes()) != "hell" {
		t.Errorf("bytes = %q, want hell", b.Bytes())
	}
}

func TestCappedBuffer_writesAfterOverflow(t *testing.T) {
	b := &cappedBuffer{limit: 3}
	b.Write([]byte("abcd")) // overflows at limit 3
	before := string(b.Bytes())
	b.Write([]byte("more data")) // should be silently dropped
	after := string(b.Bytes())
	if before != after {
		t.Errorf("buffer changed after overflow: %q -> %q", before, after)
	}
}

func TestCappedBuffer_partialFill(t *testing.T) {
	b := &cappedBuffer{limit: 5}
	b.Write([]byte("ab"))    // 2 bytes
	b.Write([]byte("cdefg")) // 5 more, but only 3 fit
	if !b.Overflowed() {
		t.Error("expected overflow after second write")
	}
	if string(b.Bytes()) != "abcde" {
		t.Errorf("bytes = %q, want abcde", b.Bytes())
	}
}

// ---- buildEnv tests ----

func TestBuildEnv_setsStore(t *testing.T) {
	env := buildEnv("/my/store")
	found := false
	for _, e := range env {
		if e == "FUMI_STORE=/my/store" {
			found = true
		}
	}
	if !found {
		t.Errorf("FUMI_STORE=/my/store not found in env: %v", env)
	}
}

func TestBuildEnv_stripsInheritedFumi(t *testing.T) {
	t.Setenv("FUMI_SECRET", "should-be-removed")
	t.Setenv("FUMI_OTHER", "also-removed")

	env := buildEnv("/store")
	for _, e := range env {
		if strings.HasPrefix(e, "FUMI_SECRET") || strings.HasPrefix(e, "FUMI_OTHER") {
			t.Errorf("inherited FUMI_* variable leaked into env: %q", e)
		}
	}
}

func TestBuildEnv_keepsNonFumi(t *testing.T) {
	t.Setenv("MY_VAR", "kept")
	env := buildEnv("/store")
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, "MY_VAR=") {
			found = true
		}
	}
	if !found {
		t.Error("non-FUMI_ env var was incorrectly stripped")
	}
}

// ---- Run tests ----

func TestRun_invalidTimeout(t *testing.T) {
	script := makeScript(t, "#!/bin/sh\necho hi\n")
	_, rpcErr := Run(context.Background(), &RunParams{
		Script:    script,
		Timeout:   0,
		StoreRoot: t.TempDir(),
	})
	if rpcErr == nil {
		t.Fatal("expected RPC error for zero timeout")
	}
	if rpcErr.Data["fumiCode"] != "PROTO_INVALID_PARAMS" {
		t.Errorf("fumiCode = %v, want PROTO_INVALID_PARAMS", rpcErr.Data["fumiCode"])
	}
}

func TestRun_success(t *testing.T) {
	script := makeScript(t, "#!/bin/sh\necho hello\n")
	outcome, rpcErr := Run(context.Background(), &RunParams{
		Script:    script,
		Timeout:   5 * time.Second,
		StoreRoot: t.TempDir(),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected RPC error: %v", rpcErr)
	}
	if outcome.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", outcome.ExitCode)
	}
	if strings.TrimSpace(string(outcome.Stdout)) != "hello" {
		t.Errorf("Stdout = %q, want hello", outcome.Stdout)
	}
}

func TestRun_nonZeroExitCode(t *testing.T) {
	script := makeScript(t, "#!/bin/sh\nexit 42\n")
	outcome, rpcErr := Run(context.Background(), &RunParams{
		Script:    script,
		Timeout:   5 * time.Second,
		StoreRoot: t.TempDir(),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected RPC error: %v", rpcErr)
	}
	if outcome.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", outcome.ExitCode)
	}
}

func TestRun_stderrCaptured(t *testing.T) {
	script := makeScript(t, "#!/bin/sh\necho err >&2\n")
	outcome, rpcErr := Run(context.Background(), &RunParams{
		Script:    script,
		Timeout:   5 * time.Second,
		StoreRoot: t.TempDir(),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected RPC error: %v", rpcErr)
	}
	if strings.TrimSpace(string(outcome.Stderr)) != "err" {
		t.Errorf("Stderr = %q, want err", outcome.Stderr)
	}
}

func TestRun_payloadPassedOnStdin(t *testing.T) {
	// The script reads stdin and echoes it to stdout.
	script := makeScript(t, "#!/bin/sh\ncat\n")
	payload := []byte(`{"key":"value"}`)
	outcome, rpcErr := Run(context.Background(), &RunParams{
		Script:    script,
		Payload:   payload,
		Timeout:   5 * time.Second,
		StoreRoot: t.TempDir(),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected RPC error: %v", rpcErr)
	}
	if string(outcome.Stdout) != string(payload) {
		t.Errorf("Stdout = %q, want %q", outcome.Stdout, payload)
	}
}

func TestRun_emptyPayloadBecomesNull(t *testing.T) {
	// When Payload is empty, the script should receive "null" on stdin.
	script := makeScript(t, "#!/bin/sh\ncat\n")
	outcome, rpcErr := Run(context.Background(), &RunParams{
		Script:    script,
		Payload:   nil,
		Timeout:   5 * time.Second,
		StoreRoot: t.TempDir(),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected RPC error: %v", rpcErr)
	}
	if string(outcome.Stdout) != "null" {
		t.Errorf("Stdout = %q, want null", outcome.Stdout)
	}
}

func TestRun_stdoutTooLarge(t *testing.T) {
	// Script produces more than 768 KiB on stdout.
	size := StdoutLimit + 1
	script := makeScript(t, fmt.Sprintf("#!/bin/sh\ndd if=/dev/zero bs=%d count=1 2>/dev/null | tr '\\0' 'x'\n", size))
	_, rpcErr := Run(context.Background(), &RunParams{
		Script:    script,
		Timeout:   10 * time.Second,
		StoreRoot: t.TempDir(),
	})
	if rpcErr == nil {
		t.Fatal("expected RPC error for stdout overflow")
	}
	if rpcErr.Data["fumiCode"] != "EXEC_OUTPUT_TOO_LARGE" {
		t.Errorf("fumiCode = %v, want EXEC_OUTPUT_TOO_LARGE", rpcErr.Data["fumiCode"])
	}
	if rpcErr.Data["stream"] != "stdout" {
		t.Errorf("stream = %v, want stdout", rpcErr.Data["stream"])
	}
}

func TestRun_timeout(t *testing.T) {
	// Use a shell busy-loop (no subprocess) so SIGKILL terminates the process
	// and closes the pipes, allowing cmd.Wait() to return promptly.
	script := makeScript(t, "#!/bin/sh\nwhile true; do :; done\n")
	_, rpcErr := Run(context.Background(), &RunParams{
		Script:    script,
		Timeout:   100 * time.Millisecond,
		StoreRoot: t.TempDir(),
	})
	if rpcErr == nil {
		t.Fatal("expected RPC error for timeout")
	}
	if rpcErr.Data["fumiCode"] != "EXEC_TIMEOUT" {
		t.Errorf("fumiCode = %v, want EXEC_TIMEOUT", rpcErr.Data["fumiCode"])
	}
}

func TestRun_storeEnvSet(t *testing.T) {
	// Script prints FUMI_STORE env var to stdout.
	storeRoot := t.TempDir()
	script := makeScript(t, "#!/bin/sh\necho $FUMI_STORE\n")
	outcome, rpcErr := Run(context.Background(), &RunParams{
		Script:    script,
		Timeout:   5 * time.Second,
		StoreRoot: storeRoot,
	})
	if rpcErr != nil {
		t.Fatalf("unexpected RPC error: %v", rpcErr)
	}
	if strings.TrimSpace(string(outcome.Stdout)) != storeRoot {
		t.Errorf("FUMI_STORE in child = %q, want %q", strings.TrimSpace(string(outcome.Stdout)), storeRoot)
	}
}

func TestRun_durationMsRecorded(t *testing.T) {
	script := makeScript(t, "#!/bin/sh\nsleep 0.1\n")
	outcome, rpcErr := Run(context.Background(), &RunParams{
		Script:    script,
		Timeout:   5 * time.Second,
		StoreRoot: t.TempDir(),
	})
	if rpcErr != nil {
		t.Fatalf("unexpected RPC error: %v", rpcErr)
	}
	if outcome.DurationMs < 50 {
		t.Errorf("DurationMs = %d, expected >= 50ms", outcome.DurationMs)
	}
}
