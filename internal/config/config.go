package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds all runtime configuration for radar.
type Config struct {
	StorageDir string
	Timeout    int // seconds
}

// Load reads configuration from environment variables, applying defaults.
func Load() (*Config, error) {
	storageDir := os.Getenv("RADAR_STORAGE_DIR")
	if storageDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("config: resolve home dir: %w", err)
		}
		storageDir = filepath.Join(home, ".config", "radar")
	}

	timeout := 30
	if raw := os.Getenv("RADAR_TIMEOUT"); raw != "" {
		t, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("config: parse RADAR_TIMEOUT: %w", err)
		}
		timeout = t
	}

	return &Config{
		StorageDir: storageDir,
		Timeout:    timeout,
	}, nil
}
