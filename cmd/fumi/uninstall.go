package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

func uninstallCmd() *cli.Command {
	return &cli.Command{
		Name:  "uninstall",
		Usage: "Remove the Native Messaging manifest (the store is preserved)",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "browser", Value: "chrome", Usage: "target browser"},
			&cli.BoolFlag{Name: "all-browsers", Usage: "uninstall from every supported browser"},
			&cli.StringFlag{Name: "manifest-dir", Hidden: true},
		},
		Action: runUninstall,
	}
}

func runUninstall(c *cli.Context) error {
	w := c.App.Writer

	var browsers []string
	if c.Bool("all-browsers") {
		browsers = []string{"chrome"}
	} else {
		browsers = []string{c.String("browser")}
	}

	override := c.String("manifest-dir")
	removed := 0
	for _, b := range browsers {
		dir := override
		if dir == "" {
			d, err := manifestDirFor(b)
			if err != nil {
				fmt.Fprintf(w, "[skip] %s: %v\n", b, err)
				continue
			}
			dir = d
		}
		path := manifestPathFor(dir)
		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(w, "[skip] %s: no manifest at %s\n", b, path)
				continue
			}
			return cli.Exit(fmt.Sprintf("failed to remove %s: %v", path, err), exitDomain)
		}
		fmt.Fprintf(w, "[ok]   %s: removed %s\n", b, path)
		removed++
	}
	fmt.Fprintf(w, "\nUninstalled manifests: %d. The fumi store was not touched.\n", removed)
	return nil
}
