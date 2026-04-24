package main

import (
	"fmt"
	"io"
	"os"

	"github.com/tkuramot/fumi/internal/config"
	"github.com/tkuramot/fumi/internal/store"
	"github.com/urfave/cli/v2"
)

func doctorCmd() *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Diagnose the fumi installation",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "browser", Value: "chrome"},
		},
		Action: runDoctor,
	}
}

type checkStatus int

const (
	statusOK checkStatus = iota
	statusWarn
	statusNG
)

func (s checkStatus) tag() string {
	switch s {
	case statusOK:
		return "[OK]  "
	case statusWarn:
		return "[WARN]"
	default:
		return "[NG]  "
	}
}

func runDoctor(c *cli.Context) error {
	w := c.App.Writer
	ng := 0
	check := func(st checkStatus, msg string) {
		fmt.Fprintf(w, "%s %s\n", st.tag(), msg)
		if st == statusNG {
			ng++
		}
	}

	// Manifest location
	manifestDir, err := manifestDirFor(c.String("browser"))
	if err != nil {
		check(statusNG, fmt.Sprintf("Resolve manifest dir: %v", err))
		return finalize(w, ng)
	}
	manifestPath := manifestPathFor(manifestDir)

	m, err := readManifest(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			check(statusNG, fmt.Sprintf("Native Messaging manifest: %s (not found). Run 'fumi setup'.", manifestPath))
		} else {
			check(statusNG, fmt.Sprintf("Native Messaging manifest: %s (%v)", manifestPath, err))
		}
		return finalize(w, ng)
	}
	check(statusOK, fmt.Sprintf("Native Messaging manifest: %s", manifestPath))

	want := buildManifest()
	if sameAllowedOrigins(m.AllowedOrigins, want.AllowedOrigins) {
		check(statusOK, "allowed_origins matches embedded Extension IDs")
	} else {
		check(statusNG, "allowed_origins does NOT match embedded Extension IDs (re-run 'fumi setup --force')")
	}

	if info, err := os.Stat(m.Path); err != nil {
		check(statusNG, fmt.Sprintf("fumi-host at %s (%v)", m.Path, err))
	} else if info.Mode().Perm()&0o111 == 0 {
		check(statusNG, fmt.Sprintf("fumi-host at %s (not executable)", m.Path))
	} else {
		check(statusOK, fmt.Sprintf("fumi-host at %s (executable)", m.Path))
	}

	// Store
	_, cfgErr := config.Load()
	if cfgErr != nil {
		check(statusNG, fmt.Sprintf("config.toml: parse error (%v)", cfgErr))
	}
	paths, err := store.Resolve()
	if err != nil {
		check(statusNG, fmt.Sprintf("Resolve store root: %v", err))
		return finalize(w, ng)
	}
	root := paths.Root

	if info, err := os.Stat(root); err != nil {
		check(statusNG, fmt.Sprintf("Store: %s (%v)", root, err))
	} else {
		mode := info.Mode().Perm()
		if mode == 0o700 {
			check(statusOK, fmt.Sprintf("Store: %s (mode %#o)", root, mode))
		} else {
			check(statusWarn, fmt.Sprintf("Store: %s (mode %#o, expected 0700)", root, mode))
		}
	}

	actionsDir := root + "/actions"
	scriptsDir := root + "/scripts"
	nActions := countFiles(actionsDir, ".js")
	nScripts := countAnyFiles(scriptsDir)
	if _, err := os.Stat(actionsDir); err != nil {
		check(statusNG, fmt.Sprintf("actions/: missing (%v)", err))
	} else if _, err := os.Stat(scriptsDir); err != nil {
		check(statusNG, fmt.Sprintf("scripts/: missing (%v)", err))
	} else {
		check(statusOK, fmt.Sprintf("actions/ (%d files), scripts/ (%d files)", nActions, nScripts))
	}

	if cfgErr == nil {
		check(statusOK, "config.toml: valid")
	}

	return finalize(w, ng)
}

func finalize(w io.Writer, ng int) error {
	fmt.Fprintln(w)
	if ng == 0 {
		fmt.Fprintln(w, "All checks passed.")
		return nil
	}
	return cli.Exit(fmt.Sprintf("%d check(s) failed.", ng), exitDomain)
}

func countFiles(dir, suffix string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && (suffix == "" || hasSuffix(e.Name(), suffix)) {
			n++
		}
	}
	return n
}

func countAnyFiles(dir string) int { return countFiles(dir, "") }

func hasSuffix(s, suffix string) bool {
	if len(suffix) > len(s) {
		return false
	}
	return s[len(s)-len(suffix):] == suffix
}
