package wazuh

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ListAgents fetches all agents from /agents (page 1, limit 500).
// For MVP one page is enough; the response shape allows future paging.
func (c *Client) ListAgents(ctx context.Context) ([]Agent, error) {
	resp, err := c.doAuthed(ctx, "GET", "/agents?limit=500")
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list agents: HTTP %d", resp.StatusCode)
	}
	var ar agentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("list agents decode: %w", err)
	}
	return ar.Data.AffectedItems, nil
}

// GetAgent fetches a single agent by ID. Config retrieval is a future
// enhancement — for MVP the detail view reads only metadata.
func (c *Client) GetAgent(ctx context.Context, id string) (AgentDetail, error) {
	resp, err := c.doAuthed(ctx, "GET", "/agents?agents_list="+id)
	if err != nil {
		return AgentDetail{}, fmt.Errorf("get agent: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return AgentDetail{}, ErrAgentNotFound
	}
	if resp.StatusCode != 200 {
		return AgentDetail{}, fmt.Errorf("get agent: HTTP %d", resp.StatusCode)
	}
	var ar singleAgentResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return AgentDetail{}, fmt.Errorf("get agent decode: %w", err)
	}
	if len(ar.Data.AffectedItems) == 0 {
		return AgentDetail{}, ErrAgentNotFound
	}
	return AgentDetail{Agent: ar.Data.AffectedItems[0]}, nil
}

func neverExpire() time.Time { return time.Now().Add(24 * time.Hour) }

// ErrAgentNotFound is returned by GetAgent when the agent is gone.
var ErrAgentNotFound = fmt.Errorf("agent not found")
