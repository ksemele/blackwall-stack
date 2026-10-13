package wazuh

import "encoding/json"

// The Wazuh 4.x syscollector API nests related fields — cpu.cores, ram.total,
// os.name, local.ip, rx.bytes, scan.time — while the Go types keep them flat,
// which is what the views want to read. These decoders bridge the two shapes.
// Each one decodes the plain fields through an alias type (no recursion) and
// the nested objects through a side struct.

type scanInfo struct {
	Time string `json:"time"`
}

func (h *Hardware) UnmarshalJSON(b []byte) error {
	type plain Hardware
	var nested struct {
		CPU struct {
			Name  string  `json:"name"`
			Cores int     `json:"cores"`
			MHz   float64 `json:"mhz"`
		} `json:"cpu"`
		RAM struct {
			Total int64   `json:"total"`
			Free  int64   `json:"free"`
			Usage float64 `json:"usage"`
		} `json:"ram"`
		Scan scanInfo `json:"scan"`
	}
	if err := json.Unmarshal(b, (*plain)(h)); err != nil {
		return err
	}
	if err := json.Unmarshal(b, &nested); err != nil {
		return err
	}
	h.CPUName, h.CPUCores, h.CPUMHz = nested.CPU.Name, nested.CPU.Cores, nested.CPU.MHz
	h.RAMTotal, h.RAMFree, h.RAMUsage = nested.RAM.Total, nested.RAM.Free, nested.RAM.Usage
	h.ScanTime = nested.Scan.Time
	return nil
}

func (o *OSInfo) UnmarshalJSON(b []byte) error {
	type plain OSInfo
	var nested struct {
		OS struct {
			Name     string `json:"name"`
			Version  string `json:"version"`
			Major    string `json:"major"`
			Minor    string `json:"minor"`
			Patch    string `json:"patch"`
			Platform string `json:"platform"`
			Codename string `json:"codename"`
		} `json:"os"`
		Scan scanInfo `json:"scan"`
	}
	if err := json.Unmarshal(b, (*plain)(o)); err != nil {
		return err
	}
	if err := json.Unmarshal(b, &nested); err != nil {
		return err
	}
	o.OSName, o.OSVersion = nested.OS.Name, nested.OS.Version
	o.OSMajor, o.OSMinor, o.OSPatch = nested.OS.Major, nested.OS.Minor, nested.OS.Patch
	o.OSPlatform, o.OSCodename = nested.OS.Platform, nested.OS.Codename
	o.ScanTime = nested.Scan.Time
	return nil
}

type ifaceCounters struct {
	Bytes   int64 `json:"bytes"`
	Packets int64 `json:"packets"`
	Errors  int64 `json:"errors"`
	Dropped int64 `json:"dropped"`
}

func (n *Netiface) UnmarshalJSON(b []byte) error {
	type plain Netiface
	var nested struct {
		RX ifaceCounters `json:"rx"`
		TX ifaceCounters `json:"tx"`
	}
	if err := json.Unmarshal(b, (*plain)(n)); err != nil {
		return err
	}
	if err := json.Unmarshal(b, &nested); err != nil {
		return err
	}
	n.RxBytes, n.RxPackets, n.RxErrors, n.RxDropped = nested.RX.Bytes, nested.RX.Packets, nested.RX.Errors, nested.RX.Dropped
	n.TxBytes, n.TxPackets, n.TxErrors, n.TxDropped = nested.TX.Bytes, nested.TX.Packets, nested.TX.Errors, nested.TX.Dropped
	return nil
}

type endpoint struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

func (p *Port) UnmarshalJSON(b []byte) error {
	type plain Port
	var nested struct {
		Local  endpoint `json:"local"`
		Remote endpoint `json:"remote"`
	}
	if err := json.Unmarshal(b, (*plain)(p)); err != nil {
		return err
	}
	if err := json.Unmarshal(b, &nested); err != nil {
		return err
	}
	p.LocalIP, p.LocalPort = nested.Local.IP, nested.Local.Port
	p.RemoteIP, p.RemotePort = nested.Remote.IP, nested.Remote.Port
	return nil
}
