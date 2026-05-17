// Package config models the blackwall YAML configuration.
package config

import "time"

type Config struct {
	Current              string             `yaml:"current"`
	SelectContextOnStart bool               `yaml:"select_context_on_start"`
	Contexts             map[string]Context `yaml:"contexts"`
	UI                   UI                 `yaml:"ui"`
	Logging              Logging            `yaml:"logging"`
}

type Context struct {
	Description string          `yaml:"description"`
	Wazuh       *WazuhSubsystem `yaml:"wazuh,omitempty"`
}

type WazuhSubsystem struct {
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	TLS      TLS    `yaml:"tls"`
}

type TLS struct {
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	CAFile             string `yaml:"ca_file"`
}

type UI struct {
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	Theme           string        `yaml:"theme"`
	VimKeys         bool          `yaml:"vim_keys"`
}

type Logging struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}
