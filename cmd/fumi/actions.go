package main

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/tkuramot/fumi/internal/config"
	"github.com/tkuramot/fumi/internal/store"
	"github.com/urfave/cli/v2"
)

func actionsCmd() *cli.Command {
	return &cli.Command{
		Name:  "actions",
		Usage: "Manage user-script actions",
		Subcommands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "List all actions discovered in the store",
				Action: runActionsList,
			},
		},
	}
}

func runActionsList(c *cli.Context) error {
	cfg, _ := config.Load()
	paths, err := store.Resolve(cfg)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to resolve store: %v", err), exitDomain)
	}
	actions, perFile, err := store.LoadAll(paths)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to read %s: %v", paths.Actions, err), exitDomain)
	}

	w := tabwriter.NewWriter(c.App.Writer, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPATH\tMATCHES")
	for _, a := range actions {
		fmt.Fprintf(w, "%s\t%s\t%s\n", a.ID, a.Path, strings.Join(a.Matches, ","))
	}
	w.Flush()

	for _, e := range perFile {
		fmt.Fprintf(c.App.ErrWriter, "[ERR] %s: %s\n", e.Path, e.Reason)
	}
	if len(perFile) > 0 {
		return cli.Exit("", exitDomain)
	}
	return nil
}
