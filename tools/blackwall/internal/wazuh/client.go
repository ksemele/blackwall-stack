package wazuh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

type Client struct {
	host           string
	port           int
	user           string
	password       string
	httpClient     *http.Client
	insecureScheme string // test override; production stays "https"

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

// New constructs a Client for the given Wazuh subsystem.
func New(sub config.WazuhSubsystem) (*Client, error) {
	httpc, err := buildHTTPClient(sub.TLS)
	if err != nil {
		return nil, err
	}
	return &Client{
		host:           sub.Server,
		port:           sub.Port,
		user:           sub.User,
		password:       sub.Password,
		httpClient:     httpc,
		insecureScheme: "https",
	}, nil
}

func (c *Client) baseURL() string {
	return fmt.Sprintf("%s://%s:%d", c.insecureScheme, c.host, c.port)
}

func (c *Client) Authenticate(ctx context.Context) error {
	url := c.baseURL() + "/security/user/authenticate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.user, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authenticate: HTTP %d", resp.StatusCode)
	}
	var ar authResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return fmt.Errorf("authenticate decode: %w", err)
	}
	if strings.TrimSpace(ar.Data.Token) == "" {
		return fmt.Errorf("authenticate: empty token")
	}

	c.mu.Lock()
	c.token = ar.Data.Token
	c.tokenExp = time.Now().Add(13 * time.Minute) // Wazuh JWT ~15min; refresh early
	c.mu.Unlock()
	return nil
}

// doAuthed runs an HTTP request with the JWT, auto-authenticating
// on first call and once on 401.
func (c *Client) doAuthed(ctx context.Context, method, path string) (*http.Response, error) {
	if err := c.ensureToken(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, nil)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	req.Header.Set("Authorization", "Bearer "+c.token)
	c.mu.Unlock()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		c.invalidateToken()
		if err := c.Authenticate(ctx); err != nil {
			return nil, err
		}
		// Retry once.
		req2, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, nil)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		req2.Header.Set("Authorization", "Bearer "+c.token)
		c.mu.Unlock()
		return c.httpClient.Do(req2)
	}
	return resp, nil
}

func (c *Client) ensureToken(ctx context.Context) error {
	c.mu.Lock()
	stale := c.token == "" || time.Now().After(c.tokenExp)
	c.mu.Unlock()
	if !stale {
		return nil
	}
	return c.Authenticate(ctx)
}

func (c *Client) invalidateToken() {
	c.mu.Lock()
	c.token = ""
	c.tokenExp = time.Time{}
	c.mu.Unlock()
}
