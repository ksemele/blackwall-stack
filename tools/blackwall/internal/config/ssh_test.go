package config

import (
	"strings"
	"testing"
	"time"
)

const sshYAML = `
current: prod
contexts:
  prod:
    ssh:
      host: prod-manager
      command: [sshpass, -p, "${BW_TEST_SSH_PW}", ssh, -F, ssh_config]
      connect_timeout: 7s
    wazuh:
      server: 127.0.0.1
      port: 55000
      user: wazuh
      password: wazuh
    indexer:
      server: 127.0.0.1
      port: 9200
      user: admin
      password: admin
`

func loadSSH(t *testing.T, body string) (Config, error) {
	t.Helper()
	raw, err := Load(writeTempFile(t, "config.yaml", body))
	if err != nil {
		return Config{}, err
	}
	cfg, err := ExpandEnv(ApplyDefaults(raw))
	if err != nil {
		return cfg, err
	}
	return cfg, Validate(cfg)
}

func TestSSH_ParsedExpandedValidated(t *testing.T) {
	t.Setenv("BW_TEST_SSH_PW", "lab-pw")
	cfg, err := loadSSH(t, sshYAML)
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.Contexts["prod"].SSH
	if s == nil || s.Host != "prod-manager" || s.ConnectTimeout != 7*time.Second {
		t.Fatalf("ssh = %+v", s)
	}
	if len(s.Command) != 6 || s.Command[2] != "lab-pw" {
		t.Errorf("command = %q, want expanded password at [2]", s.Command)
	}
}

func TestSSH_DefaultTimeoutAndOptionalCommand(t *testing.T) {
	t.Setenv("BW_TEST_SSH_PW", "x")
	body := strings.Replace(sshYAML, "      connect_timeout: 7s\n", "", 1)
	body = strings.Replace(body, "      command: [sshpass, -p, \"${BW_TEST_SSH_PW}\", ssh, -F, ssh_config]\n", "", 1)
	cfg, err := loadSSH(t, body)
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.Contexts["prod"].SSH
	if s.ConnectTimeout != DefaultSSHConnectTimeout || s.Command != nil {
		t.Errorf("ssh = %+v, want default timeout and nil command", s)
	}
}

func TestSSH_NoBlockMeansNoTunnel(t *testing.T) {
	cfg, err := loadSSH(t, indexerYAMLNoEnv)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Contexts["lab"].SSH != nil {
		t.Error("context without ssh: must stay direct")
	}
}

const indexerYAMLNoEnv = `
current: lab
contexts:
  lab:
    wazuh: {server: 127.0.0.1, port: 55000, user: wazuh, password: wazuh}
`

func TestSSH_ValidationErrors(t *testing.T) {
	t.Setenv("BW_TEST_SSH_PW", "x")
	cases := map[string]struct{ from, to, want string }{
		"empty host":     {"host: prod-manager", `host: ""`, "ssh.host is required"},
		"option as host": {"host: prod-manager", `host: "-oProxyCommand=x"`, "must not start with '-'"},
		"empty command":  {`command: [sshpass, -p, "${BW_TEST_SSH_PW}", ssh, -F, ssh_config]`, `command: [ssh, ""]`, "ssh.command[1] is empty"},
		"negative":       {"connect_timeout: 7s", "connect_timeout: -1s", "at least 1s"},
		"bad timeout":    {"connect_timeout: 7s", "connect_timeout: soon", "time.Duration"},
		"bare number":    {"connect_timeout: 7s", "connect_timeout: 10", "time.Duration"},
		"too short":      {"connect_timeout: 7s", "connect_timeout: 500ms", "at least 1s"},
		"block w/o host": {"      host: prod-manager\n", "", "ssh.host is required"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := loadSSH(t, strings.Replace(sshYAML, c.from, c.to, 1))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want %q", err, c.want)
			}
		})
	}
}

func TestSSH_RequiresWazuh(t *testing.T) {
	body := "current: x\ncontexts:\n  x:\n    ssh: {host: h}\n"
	if _, err := loadSSH(t, body); err == nil || !strings.Contains(err.Error(), "needs a wazuh section") {
		t.Errorf("err = %v", err)
	}
}

func TestSSH_ResolveKeepsBlock(t *testing.T) {
	cfg := Config{Current: "a", Contexts: map[string]Context{
		"a": {}, "b": {SSH: &SSHTunnel{Host: "h", ConnectTimeout: time.Second}},
	}}
	got, err := Resolve(cfg, Overrides{Context: "b", ContextWasSet: true}, EnvValues{})
	if err != nil {
		t.Fatal(err)
	}
	if s := got.Contexts[got.Current].SSH; s == nil || s.Host != "h" {
		t.Errorf("resolved ssh = %+v", s)
	}
}
