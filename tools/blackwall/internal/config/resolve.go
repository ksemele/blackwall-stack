package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Overrides holds values that come from CLI flags. Pointers distinguish
// "user supplied" from "default zero value".
type Overrides struct {
	Context              string
	ContextWasSet        bool
	Refresh              *time.Duration
	SelectContextOnStart *bool
}

// EnvValues holds parsed environment overrides.
type EnvValues struct {
	Context              string
	Refresh              *time.Duration
	SelectContextOnStart *bool
}

// ReadEnv reads blackwall-related env vars into EnvValues.
func ReadEnv() EnvValues {
	out := EnvValues{Context: os.Getenv("BW_CONTEXT")}
	if v := os.Getenv("BW_REFRESH"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			out.Refresh = &d
		}
	}
	if v := os.Getenv("BW_SELECT_CONTEXT_ON_START"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			out.SelectContextOnStart = &b
		}
	}
	return out
}

// Resolve folds CLI overrides and env values into a Config.
// Precedence (highest wins): CLI > env > config file > defaults.
// Callers are expected to have already run ApplyDefaults, ExpandEnv and Validate.
func Resolve(cfg Config, ov Overrides, env EnvValues) (Config, error) {
	// Context: CLI > env > file
	switch {
	case ov.ContextWasSet:
		cfg.Current = ov.Context
		ov.SelectContextOnStart = boolPtr(false) // explicit --context kills picker
	case env.Context != "":
		cfg.Current = env.Context
		ov.SelectContextOnStart = boolPtr(false)
	}

	// Refresh: CLI > env > file
	switch {
	case ov.Refresh != nil:
		cfg.UI.RefreshInterval = *ov.Refresh
	case env.Refresh != nil:
		cfg.UI.RefreshInterval = *env.Refresh
	}

	// Picker toggle: CLI > env > file
	switch {
	case ov.SelectContextOnStart != nil:
		cfg.SelectContextOnStart = *ov.SelectContextOnStart
	case env.SelectContextOnStart != nil:
		cfg.SelectContextOnStart = *env.SelectContextOnStart
	}

	if cfg.Current == "" {
		return cfg, fmt.Errorf("no current context after resolve")
	}
	if _, ok := cfg.Contexts[cfg.Current]; !ok {
		return cfg, fmt.Errorf("context %q not found in config", cfg.Current)
	}
	return cfg, nil
}

func boolPtr(b bool) *bool { return &b }
