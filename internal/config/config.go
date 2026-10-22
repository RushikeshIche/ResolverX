package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the application and DNS resolver configuration.
type Config struct {
	ListenAddress   string   `json:"listen_address"`
	UpstreamServers []string `json:"upstream_servers"`
	CacheEnabled    bool     `json:"cache_enabled"`
	CacheMaxEntries int      `json:"cache_max_entries"`
	LogQueries      bool     `json:"log_queries"`
	DatabasePath    string   `json:"database_path"`
}

// DefaultConfig returns the default configuration for ResolverX.
func DefaultConfig() *Config {
	return &Config{
		ListenAddress: "127.0.0.1:5354",
		UpstreamServers: []string{
			"1.1.1.1:53",
			"8.8.8.8:53",
		},
		CacheEnabled:    true,
		CacheMaxEntries: 10000,
		LogQueries:      true,
		DatabasePath:    "resolverx.db",
	}
}

// LoadConfig loads the configuration from the given file path.
// If the file does not exist, it saves and returns the default configuration.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("config path cannot be empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			def := DefaultConfig()
			if saveErr := SaveConfig(path, def); saveErr != nil {
				return nil, fmt.Errorf("failed to save default config: %w", saveErr)
			}
			return def, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// SaveConfig writes the configuration to the specified file path.
func SaveConfig(path string, cfg *Config) error {
	if path == "" {
		return errors.New("config path cannot be empty")
	}
	if cfg == nil {
		return errors.New("config cannot be nil")
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid config: %w", err)
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate checks whether the configuration fields are valid.
func (c *Config) Validate() error {
	if c.ListenAddress == "" {
		return errors.New("listen_address is required")
	}
	if len(c.UpstreamServers) == 0 {
		return errors.New("at least one upstream server must be configured")
	}
	if c.CacheMaxEntries <= 0 {
		c.CacheMaxEntries = 1000
	}
	if c.DatabasePath == "" {
		c.DatabasePath = "resolverx.db"
	}
	return nil
}
