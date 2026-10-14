package wazuh

import (
	"context"
	"net/http"
	"os"
	"testing"
)

// Golden tests decode real Wazuh 4.14 API responses captured from the lab
// (testdata/syscollector_*.json). The older tests encode our own structs as the
// fixture, so they could never notice that the API nests fields
// (cpu.cores, local.ip, tx.bytes, scan.time) instead of flattening them.

// goldenClient serves testdata/<file> for every non-auth request.
func goldenClient(t *testing.T, file string) *Client {
	t.Helper()
	body, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/security/user/authenticate" {
			authOK(w)
			return
		}
		_, _ = w.Write(body)
	})
	c, _ := New(subFromServer(srv, "u", "p"))
	c.insecureScheme = "http"
	return c
}

func TestGolden_Hardware(t *testing.T) {
	hw, err := goldenClient(t, "syscollector_hardware.json").GetHardware(context.Background(), "002")
	if err != nil {
		t.Fatal(err)
	}
	if hw.CPUCores != 10 || hw.CPUMHz != 2000 {
		t.Errorf("cpu = %d cores @ %v MHz, want 10 @ 2000", hw.CPUCores, hw.CPUMHz)
	}
	if hw.RAMTotal != 16425400 || hw.RAMFree == 0 || hw.RAMUsage == 0 {
		t.Errorf("ram = total %d free %d usage %v", hw.RAMTotal, hw.RAMFree, hw.RAMUsage)
	}
	if hw.ScanTime == "" {
		t.Error("scan time is empty")
	}
}

func TestGolden_OS(t *testing.T) {
	o, err := goldenClient(t, "syscollector_os.json").GetOSInfo(context.Background(), "002")
	if err != nil {
		t.Fatal(err)
	}
	if o.OSName != "Ubuntu" || o.OSPlatform != "ubuntu" || o.OSVersion == "" || o.OSCodename != "jammy" {
		t.Errorf("os = %+v", o)
	}
	if o.Hostname != "black-host-01" || o.Architecture != "aarch64" || o.ScanTime == "" {
		t.Errorf("os = %+v", o)
	}
}

func TestGolden_Netiface(t *testing.T) {
	ifs, err := goldenClient(t, "syscollector_netiface.json").ListNetiface(context.Background(), "002")
	if err != nil {
		t.Fatal(err)
	}
	if len(ifs) == 0 {
		t.Fatal("no interfaces")
	}
	if ifs[0].Name != "eth0" || ifs[0].RxBytes == 0 || ifs[0].TxBytes == 0 || ifs[0].RxPackets == 0 {
		t.Errorf("iface = %+v", ifs[0])
	}
}

func TestGolden_Ports(t *testing.T) {
	ports, err := goldenClient(t, "syscollector_ports.json").ListPorts(context.Background(), "002")
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) == 0 {
		t.Fatal("no ports")
	}
	for _, p := range ports {
		if p.LocalIP == "" || p.LocalPort == 0 {
			t.Errorf("port without local address: %+v", p)
		}
	}
}

// Processes, packages and netaddr already render correctly; these guard them
// against the same class of regression.
func TestGolden_ProcessesPackagesNetaddr(t *testing.T) {
	procs, err := goldenClient(t, "syscollector_processes.json").ListProcesses(context.Background(), "002")
	if err != nil || len(procs) == 0 || procs[0].Name == "" {
		t.Errorf("processes = %+v, err %v", procs, err)
	}
	pkgs, err := goldenClient(t, "syscollector_packages.json").ListPackages(context.Background(), "002")
	if err != nil || len(pkgs) == 0 || pkgs[0].Name == "" || pkgs[0].Version == "" {
		t.Errorf("packages = %+v, err %v", pkgs, err)
	}
	addrs, err := goldenClient(t, "syscollector_netaddr.json").ListNetaddr(context.Background(), "002")
	if err != nil || len(addrs) == 0 || addrs[0].Address == "" {
		t.Errorf("netaddr = %+v, err %v", addrs, err)
	}
}
