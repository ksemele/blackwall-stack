package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed template.yaml
var defaultTemplate []byte

// EnsureExists creates path with the embedded template when it is missing.
// Returns (created=true, nil) when the template was written, or
// (created=false, nil) when the file already existed.
func EnsureExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, fmt.Errorf("mkdir config dir: %w", err)
	}
	if err := os.WriteFile(path, defaultTemplate, 0o600); err != nil {
		return false, fmt.Errorf("write config: %w", err)
	}
	return true, nil
}
