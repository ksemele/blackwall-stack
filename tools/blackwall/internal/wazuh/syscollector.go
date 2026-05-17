package wazuh

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetHardware fetches hardware inventory for the given agent.
func (c *Client) GetHardware(ctx context.Context, id string) (Hardware, error) {
	resp, err := c.doAuthed(ctx, "GET", "/syscollector/"+id+"/hardware")
	if err != nil {
		return Hardware{}, fmt.Errorf("get hardware: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return Hardware{}, fmt.Errorf("get hardware: HTTP %d", resp.StatusCode)
	}
	var ar hardwareResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return Hardware{}, fmt.Errorf("get hardware decode: %w", err)
	}
	if len(ar.Data.AffectedItems) == 0 {
		return Hardware{}, fmt.Errorf("get hardware: no data")
	}
	return ar.Data.AffectedItems[0], nil
}

// GetOSInfo fetches OS information for the given agent.
func (c *Client) GetOSInfo(ctx context.Context, id string) (OSInfo, error) {
	resp, err := c.doAuthed(ctx, "GET", "/syscollector/"+id+"/os")
	if err != nil {
		return OSInfo{}, fmt.Errorf("get os info: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return OSInfo{}, fmt.Errorf("get os info: HTTP %d", resp.StatusCode)
	}
	var ar osInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return OSInfo{}, fmt.Errorf("get os info decode: %w", err)
	}
	if len(ar.Data.AffectedItems) == 0 {
		return OSInfo{}, fmt.Errorf("get os info: no data")
	}
	return ar.Data.AffectedItems[0], nil
}

// ListProcesses fetches running processes for the given agent (limit 500).
func (c *Client) ListProcesses(ctx context.Context, id string) ([]Process, error) {
	resp, err := c.doAuthed(ctx, "GET", "/syscollector/"+id+"/processes?limit=500")
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list processes: HTTP %d", resp.StatusCode)
	}
	var ar processesResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("list processes decode: %w", err)
	}
	return ar.Data.AffectedItems, nil
}

// ListPackages fetches installed packages for the given agent (limit 500).
func (c *Client) ListPackages(ctx context.Context, id string) ([]Package, error) {
	resp, err := c.doAuthed(ctx, "GET", "/syscollector/"+id+"/packages?limit=500")
	if err != nil {
		return nil, fmt.Errorf("list packages: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list packages: HTTP %d", resp.StatusCode)
	}
	var ar packagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("list packages decode: %w", err)
	}
	return ar.Data.AffectedItems, nil
}

// ListPorts fetches open ports for the given agent (limit 500).
func (c *Client) ListPorts(ctx context.Context, id string) ([]Port, error) {
	resp, err := c.doAuthed(ctx, "GET", "/syscollector/"+id+"/ports?limit=500")
	if err != nil {
		return nil, fmt.Errorf("list ports: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list ports: HTTP %d", resp.StatusCode)
	}
	var ar portsResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("list ports decode: %w", err)
	}
	return ar.Data.AffectedItems, nil
}

// ListNetiface fetches network interfaces for the given agent (limit 500).
func (c *Client) ListNetiface(ctx context.Context, id string) ([]Netiface, error) {
	resp, err := c.doAuthed(ctx, "GET", "/syscollector/"+id+"/netiface?limit=500")
	if err != nil {
		return nil, fmt.Errorf("list netiface: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list netiface: HTTP %d", resp.StatusCode)
	}
	var ar netifaceResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("list netiface decode: %w", err)
	}
	return ar.Data.AffectedItems, nil
}

// ListNetaddr fetches network addresses for the given agent (limit 500).
func (c *Client) ListNetaddr(ctx context.Context, id string) ([]Netaddr, error) {
	resp, err := c.doAuthed(ctx, "GET", "/syscollector/"+id+"/netaddr?limit=500")
	if err != nil {
		return nil, fmt.Errorf("list netaddr: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list netaddr: HTTP %d", resp.StatusCode)
	}
	var ar netaddrResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("list netaddr decode: %w", err)
	}
	return ar.Data.AffectedItems, nil
}
