package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFrom_missingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.toml")
	c, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil config for missing file")
	}
	// Path should be cleared when file is missing.
	if c.Path != "" {
		t.Errorf("Path = %q, want empty for missing file", c.Path)
	}
}

func TestLoadFrom_validFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`default_timeout_ms = 5000`+"\n"), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	c, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.DefaultTimeoutMs != 5000 {
		t.Errorf("DefaultTimeoutMs = %d, want 5000", c.DefaultTimeoutMs)
	}
	if c.Path != path {
		t.Errorf("Path = %q, want %q", c.Path, path)
	}
}

func TestLoadFrom_invalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`not valid toml =[[[`), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Error("expected error for invalid TOML, got nil")
	}
}

func TestDefaultTimeout_nil(t *testing.T) {
	var c *Config
	d := c.DefaultTimeout()
	if d != 30*time.Second {
		t.Errorf("nil Config.DefaultTimeout() = %v, want 30s", d)
	}
}

func TestDefaultTimeout_zero(t *testing.T) {
	c := &Config{DefaultTimeoutMs: 0}
	d := c.DefaultTimeout()
	if d != 30*time.Second {
		t.Errorf("zero DefaultTimeoutMs.DefaultTimeout() = %v, want 30s", d)
	}
}

func TestDefaultTimeout_negative(t *testing.T) {
	c := &Config{DefaultTimeoutMs: -100}
	d := c.DefaultTimeout()
	if d != 30*time.Second {
		t.Errorf("negative DefaultTimeoutMs.DefaultTimeout() = %v, want 30s", d)
	}
}

func TestDefaultTimeout_set(t *testing.T) {
	c := &Config{DefaultTimeoutMs: 10000}
	d := c.DefaultTimeout()
	if d != 10*time.Second {
		t.Errorf("DefaultTimeout() = %v, want 10s", d)
	}
}

func TestDefaultTimeout_smallValue(t *testing.T) {
	c := &Config{DefaultTimeoutMs: 500}
	d := c.DefaultTimeout()
	if d != 500*time.Millisecond {
		t.Errorf("DefaultTimeout() = %v, want 500ms", d)
	}
}
