package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithoutAuthToken(t *testing.T) {
	root := t.TempDir()
	mediaDir := filepath.Join(root, "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		t.Fatalf("failed to create media dir: %v", err)
	}

	configPath := filepath.Join(root, "config.toml")
	body := `media = "` + filepath.ToSlash(mediaDir) + `"`
	if err := os.WriteFile(configPath, []byte(body), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("expected config to load without auth_token, got %v", err)
	}
	if cfg.AuthToken != "" {
		t.Fatalf("expected empty auth token, got %q", cfg.AuthToken)
	}
}

func TestLoadAuthToken(t *testing.T) {
	root := t.TempDir()
	mediaDir := filepath.Join(root, "media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		t.Fatalf("failed to create media dir: %v", err)
	}

	configPath := filepath.Join(root, "config.toml")
	body := `media = "` + filepath.ToSlash(mediaDir) + `"
auth_token = "test-secret"
`
	if err := os.WriteFile(configPath, []byte(body), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("expected config to load, got %v", err)
	}
	if cfg.AuthToken != "test-secret" {
		t.Fatalf("expected auth token to load, got %q", cfg.AuthToken)
	}
}
