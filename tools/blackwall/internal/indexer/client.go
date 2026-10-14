package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/httpclient"
)

// Client talks to the Wazuh indexer's search API.
type Client struct {
	host, user, password string
	port                 int
	scheme               string // "https"; tests switch to "http"
	http                 *http.Client
}

// New builds a Client from the indexer subsystem config.
func New(sub config.IndexerSubsystem) (*Client, error) {
	h, err := httpclient.New(sub.TLS)
	if err != nil {
		return nil, err
	}
	return &Client{host: sub.Server, port: sub.Port, user: sub.User, password: sub.Password, scheme: "https", http: h}, nil
}

// Addr is "host:port" for the status line.
func (c *Client) Addr() string { return c.host + ":" + strconv.Itoa(c.port) }

type searchResponse struct {
	Hits struct {
		Hits []struct {
			Source Alert `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// SearchAlerts runs q against wazuh-alerts-* and returns hits newest first.
// A pattern that matches no index yields an empty slice, not an error.
func (c *Client) SearchAlerts(ctx context.Context, q AlertQuery) ([]Alert, error) {
	body, err := json.Marshal(BuildSearch(q))
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s://%s/wazuh-alerts-*/_search", c.scheme, c.Addr())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.user, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("indexer search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return nil, fmt.Errorf("indexer search: HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(msg))
	}
	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("indexer search decode: %w", err)
	}
	out := make([]Alert, 0, len(sr.Hits.Hits))
	for _, h := range sr.Hits.Hits {
		out = append(out, h.Source)
	}
	return out, nil
}
