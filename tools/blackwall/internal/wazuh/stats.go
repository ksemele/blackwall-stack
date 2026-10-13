package wazuh

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetAgentStats fetches agent-level event statistics for the given agent.
func (c *Client) GetAgentStats(ctx context.Context, id string) (AgentStats, error) {
	resp, err := c.doAuthed(ctx, "GET", "/agents/"+id+"/stats/agent")
	if err != nil {
		return AgentStats{}, fmt.Errorf("get agent stats: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return AgentStats{}, fmt.Errorf("get agent stats: HTTP %d", resp.StatusCode)
	}
	var ar agentStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return AgentStats{}, fmt.Errorf("get agent stats decode: %w", err)
	}
	if len(ar.Data.AffectedItems) == 0 {
		return AgentStats{}, fmt.Errorf("get agent stats: no data")
	}
	return ar.Data.AffectedItems[0], nil
}

// GetLogcollectorStats fetches logcollector statistics for the given agent.
func (c *Client) GetLogcollectorStats(ctx context.Context, id string) (LogcollectorStats, error) {
	resp, err := c.doAuthed(ctx, "GET", "/agents/"+id+"/stats/logcollector")
	if err != nil {
		return LogcollectorStats{}, fmt.Errorf("get logcollector stats: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return LogcollectorStats{}, fmt.Errorf("get logcollector stats: HTTP %d", resp.StatusCode)
	}
	var ar logcollectorStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return LogcollectorStats{}, fmt.Errorf("get logcollector stats decode: %w", err)
	}
	if len(ar.Data.AffectedItems) == 0 {
		return LogcollectorStats{}, fmt.Errorf("get logcollector stats: no data")
	}
	return ar.Data.AffectedItems[0], nil
}
