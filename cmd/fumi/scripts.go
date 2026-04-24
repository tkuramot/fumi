package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/tkuramot/fumi/internal/protocol"
	"github.com/tkuramot/fumi/internal/runner"
	"github.com/tkuramot/fumi/internal/store"
	"github.com/urfave/cli/v2"
)

func scriptsCmd() *cli.Command {
	return &cli.Command{
		Name:  "scripts",
		Usage: "List and invoke external scripts",
		Subcommands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "Enumerate scripts/ recursively",
				Action: runScriptsList,
			},
			{
				Name:      "run",
				Usage:     "Invoke an external script locally (debug aid)",
				ArgsUsage: "<script-relative-path>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "payload", Usage: "JSON to pipe into stdin"},
					&cli.IntFlag{Name: "timeout", Value: 30000, Usage: "timeout in milliseconds"},
					&cli.BoolFlag{Name: "json", Usage: "emit the result as JSON"},
					&cli.BoolFlag{Name: "propagate-exit", Usage: "exit with the script's exit code"},
				},
				Action: runScriptsRun,
			},
		},
	}
}

func runScriptsList(c *cli.Context) error {
	paths, err := store.Resolve()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to resolve store: %v", err), exitDomain)
	}

	w := tabwriter.NewWriter(c.App.Writer, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PATH\tKIND\tEXEC")

	walkErr := filepath.WalkDir(paths.Scripts, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(paths.Scripts, p)
		if rerr != nil {
			rel = p
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		kind := "regular"
		suffix := ""
		if info.Mode()&os.ModeSymlink != 0 {
			kind = "symlink"
			suffix = " (will be rejected at runtime)"
		} else if !info.Mode().IsRegular() {
			kind = "other"
		}
		exec := "no"
		if info.Mode().Perm()&0o111 != 0 {
			exec = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s%s\n", rel, kind, exec, suffix)
		return nil
	})
	w.Flush()
	if walkErr != nil {
		return cli.Exit(fmt.Sprintf("failed to walk %s: %v", paths.Scripts, walkErr), exitDomain)
	}
	return nil
}

func runScriptsRun(c *cli.Context) error {
	if c.NArg() < 1 {
		return cli.Exit("usage: fumi scripts run <script-relative-path> [flags]", exitUsage)
	}
	rel := c.Args().First()

	paths, err := store.Resolve()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to resolve store: %v", err), exitDomain)
	}

	resolved, rpcErr := store.ResolveScript(paths, rel)
	if rpcErr != nil {
		return reportScriptRunError(c, rpcErr, paths, rel)
	}

	payload := json.RawMessage("null")
	if raw := c.String("payload"); raw != "" {
		if !json.Valid([]byte(raw)) {
			return cli.Exit("--payload is not valid JSON", exitUsage)
		}
		payload = json.RawMessage(raw)
	}

	timeoutMs := c.Int("timeout")
	if timeoutMs <= 0 {
		return cli.Exit("--timeout must be > 0", exitUsage)
	}

	ctx := context.Background()
	outcome, rpcErr := runner.Run(ctx, &runner.RunParams{
		Script:    resolved,
		Payload:   payload,
		Timeout:   time.Duration(timeoutMs) * time.Millisecond,
		StoreRoot: paths.Root,
	})
	if rpcErr != nil {
		return reportScriptRunError(c, rpcErr, paths, rel)
	}

	if c.Bool("json") {
		res := protocol.RunScriptResult{
			ExitCode:   outcome.ExitCode,
			Stdout:     string(outcome.Stdout),
			Stderr:     string(outcome.Stderr),
			DurationMs: outcome.DurationMs,
		}
		enc := json.NewEncoder(c.App.Writer)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return cli.Exit(fmt.Sprintf("failed to encode result: %v", err), exitInternal)
		}
	} else {
		w := c.App.Writer
		fmt.Fprintf(w, "exit: %d (%dms)\n", outcome.ExitCode, outcome.DurationMs)
		fmt.Fprintln(w, "--- stdout ---")
		if len(outcome.Stdout) == 0 {
			fmt.Fprintln(w, "(empty)")
		} else {
			w.Write(outcome.Stdout)
			if !endsWithNewline(outcome.Stdout) {
				fmt.Fprintln(w)
			}
		}
		fmt.Fprintln(w, "--- stderr ---")
		if len(outcome.Stderr) == 0 {
			fmt.Fprintln(w, "(empty)")
		} else {
			c.App.ErrWriter.Write(outcome.Stderr)
			if !endsWithNewline(outcome.Stderr) {
				fmt.Fprintln(c.App.ErrWriter)
			}
		}
	}

	if c.Bool("propagate-exit") && outcome.ExitCode != 0 {
		return cli.Exit("", outcome.ExitCode)
	}
	return nil
}

func endsWithNewline(b []byte) bool {
	return len(b) > 0 && b[len(b)-1] == '\n'
}

func reportScriptRunError(c *cli.Context, e *protocol.RpcError, paths *store.Paths, rel string) error {
	code := protocol.ErrorFumiCode(e)
	var hint string
	switch code {
	case "SCRIPT_NOT_FOUND":
		hint = fmt.Sprintf("Script not found: %s (%s)", rel, filepath.Join(paths.Scripts, rel))
	case "SCRIPT_NOT_EXECUTABLE":
		hint = fmt.Sprintf("Script is not executable. chmod +x %s", filepath.Join(paths.Scripts, rel))
	case "SCRIPT_INVALID_PATH":
		hint = fmt.Sprintf("Script path is invalid: %s (%s)", rel, e.Message)
	case "SCRIPT_NOT_REGULAR_FILE":
		hint = fmt.Sprintf("Script is a symlink or special file: %s", rel)
	}
	msg := strings.TrimSpace(fmt.Sprintf("%s: %s", code, e.Message))
	if hint != "" {
		msg = hint + "\n" + msg
	}
	return cli.Exit(msg, exitDomain)
}
