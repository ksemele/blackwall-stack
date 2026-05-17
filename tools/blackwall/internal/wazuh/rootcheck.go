package wazuh

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetRootcheckLastScan fetches the last rootcheck scan timestamps for the given agent.
func (c *Client) GetRootcheckLastScan(ctx context.Context, id string) (RootcheckScan, error) {
	resp, err := c.doAuthed(ctx, "GET", "/rootcheck/"+id+"/last_scan")
	if err != nil {
		return RootcheckScan{}, fmt.Errorf("get rootcheck last scan: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return RootcheckScan{}, fmt.Errorf("get rootcheck last scan: HTTP %d", resp.StatusCode)
	}
	var ar rootcheckScanResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return RootcheckScan{}, fmt.Errorf("get rootcheck last scan decode: %w", err)
	}
	if len(ar.Data.AffectedItems) == 0 {
		return RootcheckScan{}, fmt.Errorf("get rootcheck last scan: no data")
	}
	return ar.Data.AffectedItems[0], nil
}

// ListRootcheckFindings fetches rootcheck findings for the given agent (limit 500).
func (c *Client) ListRootcheckFindings(ctx context.Context, id string) ([]RootcheckFinding, error) {
	resp, err := c.doAuthed(ctx, "GET", "/rootcheck/"+id+"?limit=500")
	if err != nil {
		return nil, fmt.Errorf("list rootcheck findings: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list rootcheck findings: HTTP %d", resp.StatusCode)
	}
	var ar rootcheckFindingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("list rootcheck findings decode: %w", err)
	}
	return ar.Data.AffectedItems, nil
}
