package main

import (
	"fmt"
	"os"
	"runtime/debug"
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

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Fprintf(os.Stdout, "fumi-host %s\n", resolveVersion())
			os.Exit(0)
		}
	}
	// Native Messaging hosts are short-lived: process one request and exit.
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}
