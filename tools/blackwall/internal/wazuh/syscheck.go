package wazuh

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetSyscheckLastScan fetches the last syscheck scan timestamps for the given agent.
func (c *Client) GetSyscheckLastScan(ctx context.Context, id string) (SyscheckScan, error) {
	resp, err := c.doAuthed(ctx, "GET", "/syscheck/"+id+"/last_scan")
	if err != nil {
		return SyscheckScan{}, fmt.Errorf("get syscheck last scan: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return SyscheckScan{}, fmt.Errorf("get syscheck last scan: HTTP %d", resp.StatusCode)
	}
	var ar syscheckScanResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return SyscheckScan{}, fmt.Errorf("get syscheck last scan decode: %w", err)
	}
	if len(ar.Data.AffectedItems) == 0 {
		return SyscheckScan{}, fmt.Errorf("get syscheck last scan: no data")
	}
	return ar.Data.AffectedItems[0], nil
}

// ListSyscheckEntries fetches FIM entries for the given agent (limit 500).
func (c *Client) ListSyscheckEntries(ctx context.Context, id string) ([]SyscheckEntry, error) {
	resp, err := c.doAuthed(ctx, "GET", "/syscheck/"+id+"?limit=500")
	if err != nil {
		return nil, fmt.Errorf("list syscheck entries: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list syscheck entries: HTTP %d", resp.StatusCode)
	}
	var ar syscheckEntriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("list syscheck entries decode: %w", err)
	}
	return ar.Data.AffectedItems, nil
}
