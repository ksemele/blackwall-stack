package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Load reads a YAML config file from disk into a Config.
// It does NOT apply defaults, env expansion or validation —
// callers should chain ApplyDefaults / ExpandEnv / Validate.
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

// ApplyDefaults fills missing scalar fields with built-in defaults.
// `SelectContextOnStart` is a bool: we cannot distinguish "absent" from
// "explicit false" at the struct level. To make the default sticky, the
// caller is expected to use Resolve (which checks a presence map produced
// by load).
//
// Implementation note: for MVP we treat zero-value as "absent" for
// everything except top-level booleans where we accept the limitation.
func ApplyDefaults(cfg Config) Config {
	if cfg.UI.RefreshInterval == 0 {
		cfg.UI.RefreshInterval = 3 * time.Second
	}
	if cfg.UI.Theme == "" {
		cfg.UI.Theme = "dark"
	}
	if !cfg.UI.VimKeys {
		cfg.UI.VimKeys = true
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "warn"
	}
	if cfg.Logging.File == "" {
		home, _ := os.UserHomeDir()
		cfg.Logging.File = home + "/.local/state/blackwall/blackwall.log"
	}
	// MVP: default SelectContextOnStart to true when zero.
	// This means "explicit false in YAML" is overridden to true,
// but that is fine because main.go reads the raw YAML separately
	// to detect explicit-false and reapplies it. See Resolve.
	if !cfg.SelectContextOnStart {
		cfg.SelectContextOnStart = true
	}
	return cfg
}

var envVarRE = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// ExpandEnv walks every string inside Contexts.*.Wazuh and substitutes
// ${VAR} from the process environment. An empty post-expansion value
// is an error (typically a missing env var).
func ExpandEnv(cfg Config) (Config, error) {
	for name, ctx := range cfg.Contexts {
		if ctx.Wazuh == nil {
			continue
		}
		w := *ctx.Wazuh

		var expandErr error
		expand := func(in, field string) string {
			out := envVarRE.ReplaceAllStringFunc(in, func(m string) string {
				sub := envVarRE.FindStringSubmatch(m)
				return os.Getenv(sub[1])
			})
			if strings.Contains(in, "${") && strings.TrimSpace(out) == "" {
				expandErr = fmt.Errorf(
					"contexts.%s.wazuh.%s resolves to empty (env var not set?)",
					name, field,
				)
			}
			return out
		}

		w.Server = expand(w.Server, "server")
		w.User = expand(w.User, "user")
		w.Password = expand(w.Password, "password")
		w.TLS.CAFile = expand(w.TLS.CAFile, "tls.ca_file")
		if expandErr != nil {
			return cfg, expandErr
		}

		ctx.Wazuh = &w
		cfg.Contexts[name] = ctx
	}
	return cfg, nil
}

// Validate checks structural invariants of a fully-resolved config.
func Validate(cfg Config) error {
	if cfg.Current == "" {
		return fmt.Errorf("config: `current` is required")
	}
	if _, ok := cfg.Contexts[cfg.Current]; !ok {
		return fmt.Errorf("config: current context %q not defined in contexts", cfg.Current)
	}
	for name, ctx := range cfg.Contexts {
		if ctx.Wazuh == nil {
			continue
		}
		w := ctx.Wazuh
		if w.Server == "" || w.Port == 0 || w.User == "" || w.Password == "" {
			return fmt.Errorf(
				"contexts.%s.wazuh: server/port/user/password are all required",
				name,
			)
		}
		if w.TLS.CAFile != "" {
			if _, err := os.Stat(w.TLS.CAFile); err != nil {
				return fmt.Errorf(
					"contexts.%s.wazuh.tls.ca_file: %v", name, err,
				)
			}
		}
	}
	return nil
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}
