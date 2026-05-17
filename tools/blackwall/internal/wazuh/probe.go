package wazuh

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

// Probe tests whether the Wazuh manager API at sub is reachable.
// It issues a lightweight GET to /security/user/authenticate with a
// short timeout and returns nil on any HTTP response (even 401/403 —
// the manager is up). Non-nil means unreachable.
func Probe(ctx context.Context, sub config.WazuhSubsystem) error {
	httpc, err := buildHTTPClient(config.TLS{InsecureSkipVerify: sub.TLS.InsecureSkipVerify, CAFile: sub.TLS.CAFile})
	if err != nil {
		return fmt.Errorf("probe: build client: %w", err)
	}
	httpc.Timeout = 2 * time.Second

	url := fmt.Sprintf("https://%s:%d/security/user/authenticate", sub.Server, sub.Port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("probe: build request: %w", err)
	}

	resp, err := httpc.Do(req)
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}
	resp.Body.Close()
	return nil // any HTTP response means the server is up
}
