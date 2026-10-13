package wazuh

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func authOK(w http.ResponseWriter) {
	_ = json.NewEncoder(w).Encode(authResponse{Data: struct {
		Token string `json:"token"`
	}{Token: "tok"}})
}

func TestGetHardware_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscollector/001/hardware":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("Authorization = %q, want Bearer tok", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(hardwareResponse{Data: struct {
				AffectedItems []Hardware `json:"affected_items"`
			}{AffectedItems: []Hardware{
				{BoardSerial: "DMQ3", CPUName: "Intel Core i7", CPUCores: 8},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	hw, err := c.GetHardware(context.Background(), "001")
	if err != nil {
		t.Fatalf("GetHardware: %v", err)
	}
	if hw.BoardSerial != "DMQ3" || hw.CPUName != "Intel Core i7" || hw.CPUCores != 8 {
		t.Errorf("hardware = %+v", hw)
	}
}

func TestGetHardware_NonOK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	if _, err := c.GetHardware(context.Background(), "001"); err == nil {
		t.Fatal("expected error on non-200")
	}
}

func TestGetOSInfo_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscollector/001/os":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("Authorization = %q, want Bearer tok", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(osInfoResponse{Data: struct {
				AffectedItems []OSInfo `json:"affected_items"`
			}{AffectedItems: []OSInfo{
				{OSName: "Ubuntu", OSVersion: "22.04", Architecture: "x86_64"},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	info, err := c.GetOSInfo(context.Background(), "001")
	if err != nil {
		t.Fatalf("GetOSInfo: %v", err)
	}
	if info.OSName != "Ubuntu" || info.Architecture != "x86_64" {
		t.Errorf("osinfo = %+v", info)
	}
}

func TestListProcesses_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscollector/001/processes":
			if r.Header.Get("Authorization") != "Bearer tok" {
				t.Errorf("Authorization = %q, want Bearer tok", r.Header.Get("Authorization"))
			}
			if r.URL.Query().Get("limit") != "500" {
				t.Errorf("limit = %q, want 500", r.URL.Query().Get("limit"))
			}
			_ = json.NewEncoder(w).Encode(processesResponse{Data: struct {
				AffectedItems []Process `json:"affected_items"`
			}{AffectedItems: []Process{
				{PID: 1, Name: "systemd"},
				{PID: 2, Name: "kthreadd"},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	procs, err := c.ListProcesses(context.Background(), "001")
	if err != nil {
		t.Fatalf("ListProcesses: %v", err)
	}
	if len(procs) != 2 || procs[0].PID != 1 || procs[1].Name != "kthreadd" {
		t.Errorf("processes = %+v", procs)
	}
}

func TestListPackages_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscollector/001/packages":
			if r.URL.Query().Get("limit") != "500" {
				t.Errorf("limit = %q, want 500", r.URL.Query().Get("limit"))
			}
			_ = json.NewEncoder(w).Encode(packagesResponse{Data: struct {
				AffectedItems []Package `json:"affected_items"`
			}{AffectedItems: []Package{
				{Name: "bash", Version: "5.1"},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	pkgs, err := c.ListPackages(context.Background(), "001")
	if err != nil {
		t.Fatalf("ListPackages: %v", err)
	}
	if len(pkgs) != 1 || pkgs[0].Name != "bash" {
		t.Errorf("packages = %+v", pkgs)
	}
}

func TestListPorts_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscollector/001/ports":
			_ = json.NewEncoder(w).Encode(portsResponse{Data: struct {
				AffectedItems []Port `json:"affected_items"`
			}{AffectedItems: []Port{
				{Protocol: "tcp", LocalIP: "0.0.0.0", LocalPort: 22, State: "listening"},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	ports, err := c.ListPorts(context.Background(), "001")
	if err != nil {
		t.Fatalf("ListPorts: %v", err)
	}
	if len(ports) != 1 || ports[0].LocalPort != 22 {
		t.Errorf("ports = %+v", ports)
	}
}

func TestListNetiface_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscollector/001/netiface":
			_ = json.NewEncoder(w).Encode(netifaceResponse{Data: struct {
				AffectedItems []Netiface `json:"affected_items"`
			}{AffectedItems: []Netiface{
				{Name: "eth0", MAC: "aa:bb:cc:dd:ee:ff", State: "up"},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	ifaces, err := c.ListNetiface(context.Background(), "001")
	if err != nil {
		t.Fatalf("ListNetiface: %v", err)
	}
	if len(ifaces) != 1 || ifaces[0].Name != "eth0" {
		t.Errorf("netiface = %+v", ifaces)
	}
}

func TestListNetaddr_OK(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/security/user/authenticate":
			authOK(w)
		case "/syscollector/001/netaddr":
			_ = json.NewEncoder(w).Encode(netaddrResponse{Data: struct {
				AffectedItems []Netaddr `json:"affected_items"`
			}{AffectedItems: []Netaddr{
				{Iface: "eth0", Proto: "ipv4", Address: "192.168.1.10"},
			}}})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"

	addrs, err := c.ListNetaddr(context.Background(), "001")
	if err != nil {
		t.Fatalf("ListNetaddr: %v", err)
	}
	if len(addrs) != 1 || addrs[0].Address != "192.168.1.10" {
		t.Errorf("netaddr = %+v", addrs)
	}
}
