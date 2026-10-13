package wazuh

import (
	"context"
	"encoding/json"
	"fmt"
)

// ListSCAPolicies fetches SCA policy summaries for the given agent.
func (c *Client) ListSCAPolicies(ctx context.Context, id string) ([]SCAPolicy, error) {
	resp, err := c.doAuthed(ctx, "GET", "/sca/"+id)
	if err != nil {
		return nil, fmt.Errorf("list sca policies: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list sca policies: HTTP %d", resp.StatusCode)
	}
	var ar scaPoliciesResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("list sca policies decode: %w", err)
	}
	return ar.Data.AffectedItems, nil
}

// ListSCAChecks fetches SCA check results for the given agent and policy.
func (c *Client) ListSCAChecks(ctx context.Context, id, policyID string) ([]SCACheck, error) {
	resp, err := c.doAuthed(ctx, "GET", "/sca/"+id+"/checks/"+policyID)
	if err != nil {
		return nil, fmt.Errorf("list sca checks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list sca checks: HTTP %d", resp.StatusCode)
	}
	var ar scaChecksResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("list sca checks decode: %w", err)
	}
	return ar.Data.AffectedItems, nil
}
