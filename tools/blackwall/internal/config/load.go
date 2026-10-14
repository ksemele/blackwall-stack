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
	for name, ctx := range cfg.Contexts {
		if ctx.Indexer == nil {
			continue
		}
		idx := *ctx.Indexer
		if idx.Alerts.MinLevel == nil {
			lvl := DefaultMinLevel
			idx.Alerts.MinLevel = &lvl
		}
		if idx.Alerts.ExcludeGroups == nil {
			idx.Alerts.ExcludeGroups = DefaultExcludeGroups()
		}
		ctx.Indexer = &idx
		cfg.Contexts[name] = ctx
	}
	return cfg
}

var envVarRE = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// ExpandEnv walks every string inside Contexts.*.Wazuh and
// Contexts.*.Indexer and substitutes ${VAR} from the process environment.
// An empty post-expansion value is an error (typically a missing env var).
func ExpandEnv(cfg Config) (Config, error) {
	for name, ctx := range cfg.Contexts {
		var expandErr error
		expand := func(in, field string) string {
			out := envVarRE.ReplaceAllStringFunc(in, func(m string) string {
				sub := envVarRE.FindStringSubmatch(m)
				return os.Getenv(sub[1])
			})
			if strings.Contains(in, "${") && strings.TrimSpace(out) == "" && expandErr == nil {
				expandErr = fmt.Errorf(
					"contexts.%s.%s resolves to empty (env var not set?)",
					name, field,
				)
			}
			return out
		}

		if ctx.Wazuh != nil {
			w := *ctx.Wazuh
			w.Server = expand(w.Server, "wazuh.server")
			w.User = expand(w.User, "wazuh.user")
			w.Password = expand(w.Password, "wazuh.password")
			w.TLS.CAFile = expand(w.TLS.CAFile, "wazuh.tls.ca_file")
			ctx.Wazuh = &w
		}
		if ctx.Indexer != nil {
			x := *ctx.Indexer
			x.Server = expand(x.Server, "indexer.server")
			x.User = expand(x.User, "indexer.user")
			x.Password = expand(x.Password, "indexer.password")
			x.TLS.CAFile = expand(x.TLS.CAFile, "indexer.tls.ca_file")
			ctx.Indexer = &x
		}
		if expandErr != nil {
			return cfg, expandErr
		}
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
		if w := ctx.Wazuh; w != nil {
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
		if x := ctx.Indexer; x != nil {
			if x.Server == "" || x.Port == 0 || x.User == "" || x.Password == "" {
				return fmt.Errorf("contexts.%s.indexer: server/port/user/password are all required", name)
			}
			if x.TLS.CAFile != "" {
				if _, err := os.Stat(x.TLS.CAFile); err != nil {
					return fmt.Errorf("contexts.%s.indexer.tls.ca_file: %v", name, err)
				}
			}
			if l := x.Alerts.MinLevel; l != nil && (*l < 0 || *l > 15) {
				return fmt.Errorf("contexts.%s.indexer.alerts.min_level: %d is outside 0–15", name, *l)
			}
		}
	}
	return nil
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

// SetCurrent updates the `current` field in the YAML config file on disk,
// preserving all other content. It reads the file, unmarshals into a
// yaml.Node tree to keep formatting, sets current, then re-marshals.
func SetCurrent(path, contextName string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config setcurrent read: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("config setcurrent parse: %w", err)
	}

	// doc is a DocumentNode; its first child is the mapping.
	if len(doc.Content) == 0 {
		return fmt.Errorf("config setcurrent: empty document")
	}
	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return fmt.Errorf("config setcurrent: expected mapping at root")
	}

	// Walk key-value pairs; keys are at even indices.
	found := false
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == "current" {
			mapping.Content[i+1].Value = contextName
			found = true
			break
		}
	}
	if !found {
		// Key absent — append it.
		key := &yaml.Node{Kind: yaml.ScalarNode, Value: "current"}
		val := &yaml.Node{Kind: yaml.ScalarNode, Value: contextName}
		mapping.Content = append(mapping.Content, key, val)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("config setcurrent marshal: %w", err)
	}
	return os.WriteFile(path, out, 0600)
}
