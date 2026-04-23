package main

import "os"

func main() {
	// Native Messaging hosts are short-lived: process one request and exit.
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}
