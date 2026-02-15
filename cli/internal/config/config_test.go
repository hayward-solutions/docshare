package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_Path(t *testing.T) {
	t.Run("returns path within user config dir", func(t *testing.T) {
		path, err := Path()
		if err != nil {
			t.Fatalf("Path() returned error: %v", err)
		}

		if path == "" {
			t.Error("expected non-empty path")
		}

		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			t.Fatalf("UserConfigDir() returned error: %v", err)
		}

		expectedSuffix := filepath.Join(dirName, fileName)
		if filepath.Base(path) != fileName {
			t.Errorf("expected filename %s, got %s", fileName, filepath.Base(path))
		}
		if filepath.Dir(path) != filepath.Join(userConfigDir, dirName) {
			t.Errorf("expected path dir %s, got %s", filepath.Join(userConfigDir, dirName), filepath.Dir(path))
		}
		_ = expectedSuffix
	})
}

func TestConfig_Load(t *testing.T) {
	originalPath, _ := Path()

	t.Run("returns default config when file does not exist", func(t *testing.T) {
		tempDir := t.TempDir()
		testConfigPath := filepath.Join(tempDir, dirName, fileName)

		if err := os.MkdirAll(filepath.Dir(testConfigPath), 0755); err != nil {
			t.Fatalf("failed to create config dir: %v", err)
		}

		cfg := &Config{ServerURL: DefaultURL, Token: ""}
		data, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(testConfigPath, data, 0600); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}
		os.Remove(testConfigPath)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() returned error: %v", err)
		}

		if cfg.ServerURL != DefaultURL {
			t.Errorf("expected ServerURL %s, got %s", DefaultURL, cfg.ServerURL)
		}
	})

	t.Run("returns saved config from file", func(t *testing.T) {
		path, _ := Path()
		configDir := filepath.Dir(path)

		originalData, _ := os.ReadFile(path)
		defer func() {
			if originalData != nil {
				_ = os.MkdirAll(configDir, 0755)
				_ = os.WriteFile(path, originalData, 0600)
			} else {
				_ = os.Remove(path)
			}
		}()

		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("failed to create config dir: %v", err)
		}

		expectedConfig := &Config{
			ServerURL: "https://example.com",
			Token:     "test-token-123",
		}
		data, _ := json.MarshalIndent(expectedConfig, "", "  ")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() returned error: %v", err)
		}

		if cfg.ServerURL != expectedConfig.ServerURL {
			t.Errorf("expected ServerURL %s, got %s", expectedConfig.ServerURL, cfg.ServerURL)
		}
		if cfg.Token != expectedConfig.Token {
			t.Errorf("expected Token %s, got %s", expectedConfig.Token, cfg.Token)
		}
	})

	t.Run("uses default URL when server_url is empty", func(t *testing.T) {
		path, _ := Path()
		configDir := filepath.Dir(path)

		originalData, _ := os.ReadFile(path)
		defer func() {
			if originalData != nil {
				_ = os.MkdirAll(configDir, 0755)
				_ = os.WriteFile(path, originalData, 0600)
			} else {
				_ = os.Remove(path)
			}
		}()

		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("failed to create config dir: %v", err)
		}
		data := `{"server_url": "", "token": "test-token"}`
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() returned error: %v", err)
		}

		if cfg.ServerURL != DefaultURL {
			t.Errorf("expected ServerURL to default to %s, got %s", DefaultURL, cfg.ServerURL)
		}
	})

	_ = originalPath
}

func TestConfig_Save(t *testing.T) {
	t.Run("creates directory and saves config", func(t *testing.T) {
		path, _ := Path()
		originalData, _ := os.ReadFile(path)
		defer func() {
			if originalData != nil {
				_ = os.WriteFile(path, originalData, 0600)
			} else {
				_ = os.Remove(path)
			}
		}()
		_ = os.Remove(path)

		cfg := &Config{
			ServerURL: "https://api.example.com",
			Token:     "save-test-token",
		}

		if err := Save(cfg); err != nil {
			t.Fatalf("Save() returned error: %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read saved config: %v", err)
		}

		var loaded Config
		if err := json.Unmarshal(data, &loaded); err != nil {
			t.Fatalf("failed to unmarshal saved config: %v", err)
		}

		if loaded.ServerURL != cfg.ServerURL {
			t.Errorf("expected ServerURL %s, got %s", cfg.ServerURL, loaded.ServerURL)
		}
		if loaded.Token != cfg.Token {
			t.Errorf("expected Token %s, got %s", cfg.Token, loaded.Token)
		}
	})

	t.Run("sets correct file permissions", func(t *testing.T) {
		path, _ := Path()
		originalData, _ := os.ReadFile(path)
		defer func() {
			if originalData != nil {
				_ = os.WriteFile(path, originalData, 0600)
			} else {
				_ = os.Remove(path)
			}
		}()
		_ = os.Remove(path)

		cfg := &Config{ServerURL: DefaultURL, Token: "perm-test"}
		if err := Save(cfg); err != nil {
			t.Fatalf("Save() returned error: %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("failed to stat config file: %v", err)
		}

		expectedPerms := os.FileMode(filePerms)
		if info.Mode().Perm() != expectedPerms {
			t.Errorf("expected file permissions %o, got %o", expectedPerms, info.Mode().Perm())
		}
	})
}

func TestConfig_Clear(t *testing.T) {
	t.Run("removes existing config file", func(t *testing.T) {
		path, _ := Path()
		originalData, _ := os.ReadFile(path)
		defer func() {
			if originalData != nil {
				_ = os.MkdirAll(filepath.Dir(path), 0755)
				_ = os.WriteFile(path, originalData, 0600)
			}
		}()

		cfg := &Config{ServerURL: DefaultURL, Token: "clear-test"}
		if err := Save(cfg); err != nil {
			t.Fatalf("Save() returned error: %v", err)
		}

		if err := Clear(); err != nil {
			t.Fatalf("Clear() returned error: %v", err)
		}

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("expected config file to be deleted")
		}
	})

	t.Run("returns nil when file does not exist", func(t *testing.T) {
		path, _ := Path()
		originalData, _ := os.ReadFile(path)
		defer func() {
			if originalData != nil {
				_ = os.MkdirAll(filepath.Dir(path), 0755)
				_ = os.WriteFile(path, originalData, 0600)
			}
		}()

		os.Remove(path)

		err := Clear()
		if err != nil {
			t.Errorf("expected Clear() to return nil for missing file, got %v", err)
		}
	})
}

func TestConfig_HasToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{"empty token", "", false},
		{"has token", "abc123", true},
		{"whitespace token", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Token: tt.token}
			if got := cfg.HasToken(); got != tt.want {
				t.Errorf("Config.HasToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_DefaultURL(t *testing.T) {
	if DefaultURL != "http://localhost:8080" {
		t.Errorf("expected DefaultURL 'http://localhost:8080', got %s", DefaultURL)
	}
}
