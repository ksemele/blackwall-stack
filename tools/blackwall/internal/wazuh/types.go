// Package wazuh is a minimal Wazuh Manager API client for blackwall.
package wazuh

import "time"

type Agent struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	IP            string    `json:"ip"`
	Status        string    `json:"status"`
	LastKeepAlive time.Time `json:"lastKeepAlive"`
	OS            struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"os"`
	Version string    `json:"version"`
	DateAdd time.Time `json:"dateAdd"`
	Group   []string  `json:"group"`
	Manager string    `json:"manager"`
}

type AgentDetail struct {
	Agent
	Config map[string]string `json:"-"`
}

type agentsResponse struct {
	Data struct {
		AffectedItems []Agent `json:"affected_items"`
	} `json:"data"`
}

type singleAgentResponse struct {
	Data struct {
		AffectedItems []Agent `json:"affected_items"`
	} `json:"data"`
}

type authResponse struct {
	Data struct {
		Token string `json:"token"`
	} `json:"data"`
}

// Hardware holds syscollector hardware info for an agent.
type Hardware struct {
	BoardSerial string  `json:"board_serial"`
	CPUName     string  `json:"cpu_name"`
	CPUCores    int     `json:"cpu_cores"`
	CPUMHz      float64 `json:"cpu_mhz"`
	RAMTotal    int64   `json:"ram_total"`
	RAMFree     int64   `json:"ram_free"`
	RAMUsage    float64 `json:"ram_usage"`
	ScanTime    string  `json:"scan_time"`
}

// OSInfo holds syscollector OS info for an agent.
type OSInfo struct {
	Architecture string `json:"architecture"`
	Hostname     string `json:"hostname"`
	OSName       string `json:"os_name"`
	OSVersion    string `json:"os_version"`
	OSMajor      string `json:"os_major"`
	OSMinor      string `json:"os_minor"`
	OSPatch      string `json:"os_patch"`
	OSPlatform   string `json:"os_platform"`
	OSCodename   string `json:"os_codename"`
	Release      string `json:"release"`
	ScanTime     string `json:"scan_time"`
	Sysname      string `json:"sysname"`
	Version      string `json:"version"`
}

// Process holds one running process from syscollector.
// PID is declared as FlexInt because Wazuh syscollector returns it as a JSON
// string (e.g. "1") while all other numeric fields are plain integers. PPID
// stays as int — it consistently arrives unquoted in tested Wazuh versions.
type Process struct {
	PID       FlexInt `json:"pid"`
	PPID      int     `json:"ppid"`
	Name      string `json:"name"`
	Cmd       string `json:"cmd"`
	Argvs     string `json:"argvs"`
	Euser     string `json:"euser"`
	Ruser     string `json:"ruser"`
	Nlwp      int    `json:"nlwp"`
	State     string `json:"state"`
	Nice      int    `json:"nice"`
	Priority  int    `json:"priority"`
	StartTime int64  `json:"start_time"`
	TTY       int    `json:"tty"`
	VMSize    int64  `json:"vm_size"`
	Resident  int64  `json:"resident"`
	Share     int64  `json:"share"`
	Utime     int64  `json:"utime"`
	Stime     int64  `json:"stime"`
}

// Package holds one installed package from syscollector.
type Package struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Vendor      string `json:"vendor"`
	Architecture string `json:"architecture"`
	Format      string `json:"format"`
	InstallTime string `json:"install_time"`
	Description string `json:"description"`
	Size        int64  `json:"size"`
	Priority    string `json:"priority"`
	Source      string `json:"source"`
}

// Port holds one network port entry from syscollector.
// PID is FlexInt for the same reason as Process.PID: syscollector may return
// it as a quoted string even though logically it is a number.
type Port struct {
	Protocol   string  `json:"protocol"`
	LocalIP    string  `json:"local_ip"`
	LocalPort  int     `json:"local_port"`
	RemoteIP   string  `json:"remote_ip"`
	RemotePort int     `json:"remote_port"`
	TxQueue    int64   `json:"tx_queue"`
	RxQueue    int64   `json:"rx_queue"`
	Inode      int64   `json:"inode"`
	State      string  `json:"state"`
	PID        FlexInt `json:"pid"`
	Process    string  `json:"process"`
}

// Netiface holds one network interface from syscollector.
type Netiface struct {
	Name      string `json:"name"`
	Adapter   string `json:"adapter"`
	Type      string `json:"type"`
	State     string `json:"state"`
	MTU       int    `json:"mtu"`
	MAC       string `json:"mac"`
	TxPackets int64  `json:"tx_packets"`
	RxPackets int64  `json:"rx_packets"`
	TxBytes   int64  `json:"tx_bytes"`
	RxBytes   int64  `json:"rx_bytes"`
	TxErrors  int64  `json:"tx_errors"`
	RxErrors  int64  `json:"rx_errors"`
	TxDropped int64  `json:"tx_dropped"`
	RxDropped int64  `json:"rx_dropped"`
}

// Netaddr holds one network address from syscollector.
type Netaddr struct {
	Iface     string `json:"iface"`
	Proto     string `json:"proto"`
	Address   string `json:"address"`
	Netmask   string `json:"netmask"`
	Broadcast string `json:"broadcast"`
}

// SyscheckEntry holds one FIM entry from syscheck.
type SyscheckEntry struct {
	File    string `json:"file"`
	Date    string `json:"date"`
	Mtime   string `json:"mtime"`
	Type    string `json:"type"`
	Size    int64  `json:"size"`
	Perm    string `json:"perm"`
	UID     string `json:"uid"`
	GID     string `json:"gid"`
	Uname   string `json:"uname"`
	Gname   string `json:"gname"`
	MD5     string `json:"md5"`
	SHA1    string `json:"sha1"`
	SHA256  string `json:"sha256"`
	Changes int    `json:"changes"`
	Inode   int64  `json:"inode"`
}

// SyscheckScan holds the last syscheck scan timestamps.
type SyscheckScan struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// RootcheckFinding holds one rootcheck finding.
type RootcheckFinding struct {
	CIS       string `json:"cis"`
	PCIDSS    string `json:"pci_dss"`
	DateFirst string `json:"date_first"`
	DateLast  string `json:"date_last"`
	Log       string `json:"log"`
	Status    string `json:"status"`
}

// RootcheckScan holds the last rootcheck scan timestamps.
type RootcheckScan struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// SCAPolicy holds one Security Configuration Assessment policy summary.
type SCAPolicy struct {
	PolicyID    string `json:"policy_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	References  string `json:"references"`
	Pass        int    `json:"pass"`
	Fail        int    `json:"fail"`
	Score       int    `json:"score"`
	TotalChecks int    `json:"total_checks"`
	StartScan   string `json:"start_scan"`
	EndScan     string `json:"end_scan"`
}

// SCACheck holds one SCA check result.
type SCACheck struct {
	ID          int    `json:"id"`
	PolicyID    string `json:"policy_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Rationale   string `json:"rationale"`
	Remediation string `json:"remediation"`
	Result      string `json:"result"`
	Condition   string `json:"condition"`
	Compliance  string `json:"compliance"`
	References  string `json:"references"`
}

// AgentStats holds agent-level event statistics.
// TODO: verify field names against live Wazuh 4.14
type AgentStats struct {
	Events           int64   `json:"events"`
	EventsDropped    int64   `json:"events_dropped"`
	EventsPerSecond  float64 `json:"events_per_second"`
	Bytes            int64   `json:"bytes"`
}

// LogcollectorStats holds logcollector-level event statistics.
// TODO: verify field names against live Wazuh 4.14
type LogcollectorStats struct {
	Events          int64   `json:"events"`
	EventsDropped   int64   `json:"events_dropped"`
	EventsPerSecond float64 `json:"events_per_second"`
	Bytes           int64   `json:"bytes"`
}

// --- private response wrappers ---

type hardwareResponse struct {
	Data struct {
		AffectedItems []Hardware `json:"affected_items"`
	} `json:"data"`
}

type osInfoResponse struct {
	Data struct {
		AffectedItems []OSInfo `json:"affected_items"`
	} `json:"data"`
}

type processesResponse struct {
	Data struct {
		AffectedItems []Process `json:"affected_items"`
	} `json:"data"`
}

type packagesResponse struct {
	Data struct {
		AffectedItems []Package `json:"affected_items"`
	} `json:"data"`
}

type portsResponse struct {
	Data struct {
		AffectedItems []Port `json:"affected_items"`
	} `json:"data"`
}

type netifaceResponse struct {
	Data struct {
		AffectedItems []Netiface `json:"affected_items"`
	} `json:"data"`
}

type netaddrResponse struct {
	Data struct {
		AffectedItems []Netaddr `json:"affected_items"`
	} `json:"data"`
}

type syscheckEntriesResponse struct {
	Data struct {
		AffectedItems []SyscheckEntry `json:"affected_items"`
	} `json:"data"`
}

type syscheckScanResponse struct {
	Data struct {
		AffectedItems []SyscheckScan `json:"affected_items"`
	} `json:"data"`
}

type rootcheckFindingsResponse struct {
	Data struct {
		AffectedItems []RootcheckFinding `json:"affected_items"`
	} `json:"data"`
}

type rootcheckScanResponse struct {
	Data struct {
		AffectedItems []RootcheckScan `json:"affected_items"`
	} `json:"data"`
}

type scaPoliciesResponse struct {
	Data struct {
		AffectedItems []SCAPolicy `json:"affected_items"`
	} `json:"data"`
}

type scaChecksResponse struct {
	Data struct {
		AffectedItems []SCACheck `json:"affected_items"`
	} `json:"data"`
}

type agentStatsResponse struct {
	Data struct {
		AffectedItems []AgentStats `json:"affected_items"`
	} `json:"data"`
}

type logcollectorStatsResponse struct {
	Data struct {
		AffectedItems []LogcollectorStats `json:"affected_items"`
	} `json:"data"`
}
