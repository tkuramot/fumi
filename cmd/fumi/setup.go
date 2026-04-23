package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tkuramot/fumi/internal/store"
	"github.com/urfave/cli/v2"
)

func setupCmd() *cli.Command {
	return &cli.Command{
		Name:  "setup",
		Usage: "Initialize the fumi store and install the Native Messaging manifest",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "browser", Value: "chrome", Usage: "target browser"},
			&cli.BoolFlag{Name: "force", Usage: "overwrite existing manifest with a mismatching Extension ID"},
			&cli.StringFlag{Name: "store-root", Usage: "override store root (test hook)", Hidden: true},
			&cli.StringFlag{Name: "manifest-dir", Usage: "override manifest directory (test hook)", Hidden: true},
		},
		Action: runSetup,
	}
}

func runSetup(c *cli.Context) error {
	root := c.String("store-root")
	if root == "" {
		paths, err := store.Resolve(nil)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to resolve store root: %v", err), exitDomain)
		}
		root = paths.Root
	}
	if err := initStore(root); err != nil {
		return cli.Exit(fmt.Sprintf("failed to initialize store: %v", err), exitDomain)
	}

	manifestDir := c.String("manifest-dir")
	if manifestDir == "" {
		dir, err := manifestDirFor(c.String("browser"))
		if err != nil {
			return cli.Exit(err.Error(), exitDomain)
		}
		manifestDir = dir
	}
	manifestPath := manifestPathFor(manifestDir)

	if existing, err := readManifest(manifestPath); err == nil {
		want := buildManifest()
		if !sameAllowedOrigins(existing.AllowedOrigins, want.AllowedOrigins) && !c.Bool("force") {
			return cli.Exit(
				"Existing manifest points to a different Extension ID. Re-run with --force to overwrite.",
				exitDomain,
			)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return cli.Exit(fmt.Sprintf("failed to read existing manifest: %v", err), exitDomain)
	}

	if err := writeManifest(manifestPath); err != nil {
		return cli.Exit(
			fmt.Sprintf("failed to write manifest at %s: %v\nHint: is Chrome installed? Try: open -a \"Google Chrome\" once to initialize.", manifestPath, err),
			exitDomain,
		)
	}

	w := c.App.Writer
	fmt.Fprintf(w, "Store:    %s\n", root)
	fmt.Fprintf(w, "Manifest: %s\n", manifestPath)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Setup complete. Next:")
	fmt.Fprintln(w, "  1. Load the fumi Chrome extension (unpacked or from the Web Store).")
	fmt.Fprintln(w, "  2. Run 'fumi doctor' to verify the installation.")
	return nil
}

func initStore(root string) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return err
	}
	for _, sub := range []string{"actions", "scripts"} {
		p := filepath.Join(root, sub)
		if err := os.MkdirAll(p, 0o700); err != nil {
			return err
		}
		if err := os.Chmod(p, 0o700); err != nil {
			return err
		}
	}
	cfgPath := filepath.Join(root, "config.toml")
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(cfgPath, []byte(defaultConfigTOML), 0o600); err != nil {
			return err
		}
	}
	return nil
}

const defaultConfigTOML = `# fumi configuration. All fields are optional.
# store_root = "~/.config/fumi"
# default_timeout_ms = 30000
`

func sameAllowedOrigins(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]bool{}
	for _, v := range a {
		set[v] = true
	}
	for _, v := range b {
		if !set[v] {
			return false
		}
	}
	return true
}
