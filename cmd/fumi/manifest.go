package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type nativeManifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

// manifestDirFor returns the Native Messaging hosts directory for the given browser.
// Currently only Chrome on macOS is supported (spec §11.4).
func manifestDirFor(browser string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch browser {
	case "chrome", "":
		if runtime.GOOS != "darwin" {
			return "", fmt.Errorf("unsupported OS: %s (macOS only)", runtime.GOOS)
		}
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts"), nil
	default:
		return "", fmt.Errorf("unsupported browser: %s", browser)
	}
}

func manifestPathFor(dir string) string {
	return filepath.Join(dir, nativeMessagingHostName+".json")
}

func buildManifest() nativeManifest {
	return nativeManifest{
		Name:        nativeMessagingHostName,
		Description: "fumi native messaging host",
		Path:        hostBinaryPath,
		Type:        "stdio",
		AllowedOrigins: []string{
			"chrome-extension://" + webStoreExtensionID + "/",
			"chrome-extension://" + unpackedExtensionID + "/",
		},
	}
}

func writeManifest(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(buildManifest(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func readManifest(path string) (*nativeManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m nativeManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
