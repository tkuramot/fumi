package main

// These are overridden at release build time via -ldflags "-X main.<name>=...".
// Development builds fall back to placeholder values.
var (
	webStoreExtensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	unpackedExtensionID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	hostBinaryPath      = "/opt/homebrew/bin/fumi-host"
)

const nativeMessagingHostName = "com.tkuramot.fumi"
