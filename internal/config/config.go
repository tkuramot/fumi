package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	StoreRoot        string `toml:"store_root"`
	DefaultTimeoutMs int    `toml:"default_timeout_ms"`

	// Path records where the config was loaded from (empty if defaults were used).
	Path string `toml:"-"`
}

func userConfigDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		return d
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".config")
	}
	return ""
}

// DefaultPath returns the default config.toml location.
func DefaultPath() string {
	return filepath.Join(userConfigDir(), "fumi", "config.toml")
}

// Load reads ~/.config/fumi/config.toml. Missing file is not an error; defaults are returned.
func Load() (*Config, error) {
	return LoadFrom(DefaultPath())
}

func LoadFrom(path string) (*Config, error) {
	c := &Config{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.Path = ""
			return c, nil
		}
		return nil, err
	}
	if _, err := toml.Decode(string(data), c); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) DefaultTimeout() time.Duration {
	if c == nil || c.DefaultTimeoutMs <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.DefaultTimeoutMs) * time.Millisecond
}
