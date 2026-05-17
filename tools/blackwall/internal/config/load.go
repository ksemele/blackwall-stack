package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads a YAML config file from disk into a Config.
// Defaults, env expansion and validation happen elsewhere
// (see Apply / Resolve).
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// writeFile is an internal helper used by tests.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}
