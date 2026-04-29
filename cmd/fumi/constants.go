package main

// extensionID is the Chrome Web Store-assigned ID; update after first publish.
// hostBinaryPath is overridden at release time via goreleaser ldflags.
var (
	extensionID    = ""
	hostBinaryPath = "/opt/homebrew/bin/fumi-host"
)

const nativeMessagingHostName = "com.tkrmt.fumi"
