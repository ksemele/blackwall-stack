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
	Description string            `yaml:"description"`
	SSH         *SSHTunnel        `yaml:"ssh,omitempty"`
	Wazuh       *WazuhSubsystem   `yaml:"wazuh,omitempty"`
	Indexer     *IndexerSubsystem `yaml:"indexer,omitempty"`
}

// SSHTunnel reaches a loopback-only manager through the system ssh: the
// wazuh/indexer server and port are then as seen from Host.
type SSHTunnel struct {
	// Host is the ssh destination: an alias from ~/.ssh/config or user@host.
	Host string `yaml:"host"`
	// Command replaces the plain "ssh" argv prefix (e.g. sshpass ... ssh -F file).
	// When set, BatchMode is not forced, so password helpers keep working.
	Command []string `yaml:"command"`
	// ConnectTimeout bounds waiting for the forwarded ports; 0 means DefaultSSHConnectTimeout.
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
}

// DefaultSSHConnectTimeout applies when ssh.connect_timeout is omitted.
const DefaultSSHConnectTimeout = 10 * time.Second

type WazuhSubsystem struct {
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	TLS      TLS    `yaml:"tls"`
}

type IndexerSubsystem struct {
	Server   string        `yaml:"server"`
	Port     int           `yaml:"port"`
	User     string        `yaml:"user"`
	Password string        `yaml:"password"`
	TLS      TLS           `yaml:"tls"`
	Alerts   AlertDefaults `yaml:"alerts"`
}

type AlertDefaults struct {
	MinLevel      *int     `yaml:"min_level"`      // nil → DefaultMinLevel
	ExcludeGroups []string `yaml:"exclude_groups"` // nil → DefaultExcludeGroups()
}

// DefaultMinLevel hides routine events (level < 5) from the alerts feed.
const DefaultMinLevel = 5

// DefaultExcludeGroups hides CIS audit results (shown in the agent card
// already) and session snapshots (shown in the Sessions tab) from the feed.
func DefaultExcludeGroups() []string { return []string{"sca", "blackwall_sessions"} }

type TLS struct {
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	CAFile             string `yaml:"ca_file"`
	// ServerName overrides the name verified in the server certificate
	// (DNS name or IP SAN). Optional; for ssh contexts it defaults to the
	// configured server, since the client dials 127.0.0.1:<local port>.
	ServerName string `yaml:"server_name"`
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
