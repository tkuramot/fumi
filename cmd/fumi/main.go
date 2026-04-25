package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/urfave/cli/v2"
)

var version string

func resolveVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// Exit code convention (see docs/design/cli.md §5):
//   0 — success
//   1 — CLI usage error (flag parsing etc.)
//   2 — domain error (missing manifest, validation failed, doctor NG)
//   3 — internal bug
const (
	exitOK       = 0
	exitUsage    = 1
	exitDomain   = 2
	exitInternal = 3
)

func main() {
	app := &cli.App{
		Name:    "fumi",
		Usage:   "Browser × host machine integration utility",
		Version: resolveVersion(),
		Commands: []*cli.Command{
			setupCmd(),
			uninstallCmd(),
			doctorCmd(),
			actionsCmd(),
			scriptsCmd(),
		},
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			return cli.Exit(err.Error(), exitUsage)
		},
	}
	if err := app.RunContext(context.Background(), os.Args); err != nil {
		var coder cli.ExitCoder
		if errors.As(err, &coder) {
			if coder.ExitCode() != exitOK {
				fmt.Fprintln(os.Stderr, coder.Error())
			}
			os.Exit(coder.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitInternal)
	}
}
