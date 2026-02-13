package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	dirName    = "docshare"
	fileName   = "config.json"
	dirPerms   = 0700
	filePerms  = 0600
	DefaultURL = "http://localhost:8080"
)

// Config holds persisted CLI configuration.
type Config struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
}

// Path returns the full path to the config file.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, dirName, fileName), nil
}

// Load reads the config from disk. Returns a zero-value Config (not an error) if the file doesn't exist.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return &Config{ServerURL: DefaultURL}, nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{ServerURL: DefaultURL}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = DefaultURL
	}
	return &cfg, nil
}

// Save writes the config to disk, creating the directory if needed.
func Save(cfg *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), dirPerms); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, filePerms)
}

// Clear removes the config file.
func Clear() error {
	p, err := Path()
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// HasToken reports whether a token is configured.
func (c *Config) HasToken() bool {
	return c.Token != ""
}
