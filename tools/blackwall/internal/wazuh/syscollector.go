package wazuh

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// TTL constants for slow-changing syscollector data.
const (
	ttlHardware = 60 * time.Second
	ttlOSInfo   = 60 * time.Second
	ttlPackages = 60 * time.Second
	ttlNetiface = 30 * time.Second
	ttlNetaddr  = 30 * time.Second
	ttlPorts    = 30 * time.Second
)

// GetHardware fetches hardware inventory for the given agent (cached 60s).
func (c *Client) GetHardware(ctx context.Context, id string) (Hardware, error) {
	return getCached(c, "hardware:"+id, ttlHardware, func() (Hardware, error) {
		return c.fetchHardware(ctx, id)
	})
}

func (c *Client) fetchHardware(ctx context.Context, id string) (Hardware, error) {
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

// GetOSInfo fetches OS information for the given agent (cached 60s).
func (c *Client) GetOSInfo(ctx context.Context, id string) (OSInfo, error) {
	return getCached(c, "osinfo:"+id, ttlOSInfo, func() (OSInfo, error) {
		return c.fetchOSInfo(ctx, id)
	})
}

func (c *Client) fetchOSInfo(ctx context.Context, id string) (OSInfo, error) {
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
// Not cached — caller needs fresh data.
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

// ListPackages fetches installed packages for the given agent (cached 60s).
func (c *Client) ListPackages(ctx context.Context, id string) ([]Package, error) {
	return getCached(c, "packages:"+id, ttlPackages, func() ([]Package, error) {
		return c.fetchPackages(ctx, id)
	})
}

func (c *Client) fetchPackages(ctx context.Context, id string) ([]Package, error) {
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

// ListPorts fetches open ports for the given agent (cached 30s).
func (c *Client) ListPorts(ctx context.Context, id string) ([]Port, error) {
	return getCached(c, "ports:"+id, ttlPorts, func() ([]Port, error) {
		return c.fetchPorts(ctx, id)
	})
}

func (c *Client) fetchPorts(ctx context.Context, id string) ([]Port, error) {
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

// ListNetiface fetches network interfaces for the given agent (cached 30s).
func (c *Client) ListNetiface(ctx context.Context, id string) ([]Netiface, error) {
	return getCached(c, "netiface:"+id, ttlNetiface, func() ([]Netiface, error) {
		return c.fetchNetiface(ctx, id)
	})
}

func (c *Client) fetchNetiface(ctx context.Context, id string) ([]Netiface, error) {
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

// ListNetaddr fetches network addresses for the given agent (cached 30s).
func (c *Client) ListNetaddr(ctx context.Context, id string) ([]Netaddr, error) {
	return getCached(c, "netaddr:"+id, ttlNetaddr, func() ([]Netaddr, error) {
		return c.fetchNetaddr(ctx, id)
	})
}

func (c *Client) fetchNetaddr(ctx context.Context, id string) ([]Netaddr, error) {
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
