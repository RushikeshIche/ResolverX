package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rushi/ResolverX/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.ListenAddress != "127.0.0.1:5354" {
		t.Errorf("expected default listen address '127.0.0.1:5354', got '%s'", cfg.ListenAddress)
	}
	if len(cfg.UpstreamServers) != 2 {
		t.Errorf("expected 2 default upstreams, got %d", len(cfg.UpstreamServers))
	}
	if !cfg.CacheEnabled {
		t.Error("expected cache to be enabled by default")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config validation failed: %v", err)
	}
}

func TestLoadAndSaveConfig(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.json")

	// Test loading when file does not exist -> creates default
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error loading non-existent config: %v", err)
	}
	if cfg.ListenAddress != "127.0.0.1:5354" {
		t.Errorf("expected default listen address, got %s", cfg.ListenAddress)
	}

	// Verify file was written
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatal("expected config file to be created on disk")
	}

	// Modify and save
	cfg.ListenAddress = "127.0.0.1:1053"
	cfg.CacheMaxEntries = 500
	if err := config.SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Reload and verify changes
	reloaded, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if reloaded.ListenAddress != "127.0.0.1:1053" {
		t.Errorf("expected modified listen address '127.0.0.1:1053', got '%s'", reloaded.ListenAddress)
	}
	if reloaded.CacheMaxEntries != 500 {
		t.Errorf("expected cache entries 500, got %d", reloaded.CacheMaxEntries)
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ListenAddress = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty listen address, got nil")
	}

	cfg = config.DefaultConfig()
	cfg.UpstreamServers = []string{}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty upstreams, got nil")
	}
}
