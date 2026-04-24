package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/tkuramot/fumi/internal/protocol"
	"github.com/tkuramot/fumi/internal/store"
)

const (
	StdoutLimit = 768 * 1024
	StderrLimit = 128 * 1024
	killGrace   = 500 * time.Millisecond
)

type RunParams struct {
	Script    *store.ResolvedScript
	Payload   json.RawMessage
	Timeout   time.Duration
	StoreRoot string
}

type RunOutcome struct {
	ExitCode   int
	Stdout     []byte
	Stderr     []byte
	DurationMs int64
}

func buildEnv(storeRoot string) []string {
	env := os.Environ()
	// Strip any inherited FUMI_* to prevent accidental leakage; they will be re-set explicitly.
	filtered := env[:0]
	for _, e := range env {
		if strings.HasPrefix(e, "FUMI_") {
			continue
		}
		filtered = append(filtered, e)
	}
	filtered = append(filtered, "FUMI_STORE="+storeRoot)
	return filtered
}

type cappedBuffer struct {
	buf   bytes.Buffer
	limit int
	over  bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.over {
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if len(p) > remaining {
		if remaining > 0 {
			b.buf.Write(p[:remaining])
		}
		b.over = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *cappedBuffer) Overflowed() bool { return b.over }
func (b *cappedBuffer) Bytes() []byte    { return b.buf.Bytes() }

func Run(ctx context.Context, p *RunParams) (*RunOutcome, *protocol.RpcError) {
	if p.Timeout <= 0 {
		return nil, protocol.NewError("PROTO_INVALID_PARAMS", "timeout must be > 0", nil)
	}

	runCtx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	env := buildEnv(p.StoreRoot)

	cmd := exec.Command(p.Script.AbsPath)
	cmd.Dir = p.Script.Cwd
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, protocol.NewError("EXEC_SPAWN_FAILED", err.Error(), map[string]any{"scriptPath": p.Script.AbsPath})
	}

	stdout := &cappedBuffer{limit: StdoutLimit}
	stderr := &cappedBuffer{limit: StderrLimit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, protocol.NewError("EXEC_SPAWN_FAILED", err.Error(), map[string]any{"scriptPath": p.Script.AbsPath})
	}

	processExited := make(chan struct{})
	timedOut := false
	go func() {
		select {
		case <-runCtx.Done():
			if runCtx.Err() == context.DeadlineExceeded {
				timedOut = true
			}
			if cmd.Process != nil {
				_ = cmd.Process.Signal(syscall.SIGTERM)
				select {
				case <-time.After(killGrace):
					_ = cmd.Process.Kill()
				case <-processExited:
				}
			}
		case <-processExited:
		}
	}()

	writeErr := make(chan error, 1)
	go func() {
		payload := p.Payload
		if len(payload) == 0 {
			payload = []byte("null")
		}
		_, e := stdin.Write(payload)
		stdin.Close()
		writeErr <- e
	}()

	waitErr := cmd.Wait()
	close(processExited)
	duration := time.Since(start).Milliseconds()
	<-writeErr

	if stdout.Overflowed() {
		return nil, protocol.NewError("EXEC_OUTPUT_TOO_LARGE", "stdout exceeded limit",
			map[string]any{"stream": "stdout", "limitBytes": StdoutLimit})
	}
	if stderr.Overflowed() {
		return nil, protocol.NewError("EXEC_OUTPUT_TOO_LARGE", "stderr exceeded limit",
			map[string]any{"stream": "stderr", "limitBytes": StderrLimit})
	}

	if timedOut {
		return nil, protocol.NewError("EXEC_TIMEOUT", "script timed out",
			map[string]any{"timeoutMs": p.Timeout.Milliseconds(), "durationMs": duration})
	}

	exitCode := 0
	if ee, ok := waitErr.(*exec.ExitError); ok {
		exitCode = ee.ExitCode()
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			exitCode = -int(ws.Signal())
		}
	} else if waitErr != nil {
		return nil, protocol.NewError("INTERNAL", waitErr.Error(), nil)
	}

	return &RunOutcome{
		ExitCode:   exitCode,
		Stdout:     stdout.Bytes(),
		Stderr:     stderr.Bytes(),
		DurationMs: duration,
	}, nil
}
