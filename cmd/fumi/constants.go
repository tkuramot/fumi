package main

// extensionID must match the ID derived from manifest.json's "key".
// hostBinaryPath is overridden at release time via goreleaser ldflags.
var (
	extensionID    = "lcnbaehknoekfphmohakkepkdilkcnei"
	hostBinaryPath = "/opt/homebrew/bin/fumi-host"
)

const nativeMessagingHostName = "com.tkrmt.fumi"
