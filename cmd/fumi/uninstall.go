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
		},
		Action: runUninstall,
	}
}

func runUninstall(c *cli.Context) error {
	w := c.App.Writer

	browser := c.String("browser")
	dir, err := manifestDirFor(browser)
	if err != nil {
		return cli.Exit(fmt.Sprintf("%s: %v", browser, err), exitDomain)
	}
	path := manifestPathFor(dir)
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(w, "[skip] %s: no manifest at %s\n", browser, path)
			fmt.Fprintln(w, "\nThe fumi store was not touched.")
			return nil
		}
		return cli.Exit(fmt.Sprintf("failed to remove %s: %v", path, err), exitDomain)
	}
	fmt.Fprintf(w, "[ok]   %s: removed %s\n", browser, path)
	fmt.Fprintln(w, "\nThe fumi store was not touched.")
	return nil
}
