package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFromMissingReturnsDefaults(t *testing.T) {
	c, err := LoadFrom(filepath.Join(t.TempDir(), "absent.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Path != "" {
		t.Errorf("Path = %q, want empty for missing file", c.Path)
	}
	if c.DefaultTimeoutMs != 0 {
		t.Errorf("DefaultTimeoutMs = %d, want 0", c.DefaultTimeoutMs)
	}
}

func TestLoadFromValidToml(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("default_timeout_ms = 5000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Path != path {
		t.Errorf("Path = %q, want %q", c.Path, path)
	}
	if c.DefaultTimeoutMs != 5000 {
		t.Errorf("DefaultTimeoutMs = %d, want 5000", c.DefaultTimeoutMs)
	}
}

func TestLoadFromInvalidTomlErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("default_timeout_ms = \"not a number\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestDefaultTimeout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		c    *Config
		want time.Duration
	}{
		{"nil", nil, 30 * time.Second},
		{"zero", &Config{}, 30 * time.Second},
		{"negative", &Config{DefaultTimeoutMs: -1}, 30 * time.Second},
		{"explicit", &Config{DefaultTimeoutMs: 1500}, 1500 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.c.DefaultTimeout(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultPathHonorsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".config", "fumi", "config.toml")
	if got := DefaultPath(); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
