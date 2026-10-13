package agents

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// NetworkClient is the subset of wazuh.Client required by NetworkModel.
type NetworkClient interface {
	ListNetiface(ctx context.Context, id string) ([]wazuh.Netiface, error)
	ListNetaddr(ctx context.Context, id string) ([]wazuh.Netaddr, error)
	ListPorts(ctx context.Context, id string) ([]wazuh.Port, error)
}

// NetworkLoadedMsg carries the result of a network fetch.
type NetworkLoadedMsg struct {
	Interfaces []wazuh.Netiface
	Addresses  []wazuh.Netaddr
	Ports      []wazuh.Port
	Err        error
}

// NetworkModel is the Network tab sub-view.
type NetworkModel struct {
	client     NetworkClient
	agentID    string
	interfaces []wazuh.Netiface
	addresses  []wazuh.Netaddr
	ports      []wazuh.Port
	loaded     bool
	err        string
}

func NewNetwork(c NetworkClient, agentID string) *NetworkModel {
	return &NetworkModel{client: c, agentID: agentID}
}

func (m *NetworkModel) Init() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		ifaces, err := m.client.ListNetiface(ctx, m.agentID)
		if err != nil {
			return NetworkLoadedMsg{Err: err}
		}
		addrs, err := m.client.ListNetaddr(ctx, m.agentID)
		if err != nil {
			return NetworkLoadedMsg{Err: err}
		}
		ports, err := m.client.ListPorts(ctx, m.agentID)
		if err != nil {
			return NetworkLoadedMsg{Err: err}
		}
		return NetworkLoadedMsg{Interfaces: ifaces, Addresses: addrs, Ports: ports}
	}
}

func (m *NetworkModel) Update(msg tea.Msg) (subView, tea.Cmd) {
	if lm, ok := msg.(NetworkLoadedMsg); ok {
		if lm.Err != nil {
			m.err = lm.Err.Error()
			return m, nil
		}
		m.interfaces = lm.Interfaces
		m.addresses = lm.Addresses
		m.ports = lm.Ports
		m.loaded = true
		m.err = ""
	}
	return m, nil
}

func (m *NetworkModel) View() string {
	if m.err != "" {
		return styles.Error.Render("⚠ "+m.err) + "\n"
	}
	if !m.loaded {
		return styles.Dim.Render("loading…") + "\n"
	}

	var b strings.Builder

	// Interfaces
	b.WriteString(styles.HeaderText.Render("Interfaces") + "\n")
	b.WriteString("──────────\n")
	if len(m.interfaces) == 0 {
		b.WriteString("(none)\n")
	} else {
		fmt.Fprintf(&b, "%-10s  %-20s  %-8s  %-6s  %-6s  %-18s  %s\n",
			"NAME", "ADAPTER", "TYPE", "STATE", "MTU", "MAC", "TX/RX bytes")
		for _, iface := range m.interfaces {
			txrx := fmt.Sprintf("%s / %s", formatBytes(iface.TxBytes), formatBytes(iface.RxBytes))
			fmt.Fprintf(&b, "%-10s  %-20s  %-8s  %-6s  %-6d  %-18s  %s\n",
				iface.Name,
				truncate(iface.Adapter, 20),
				iface.Type,
				iface.State,
				iface.MTU,
				iface.MAC,
				txrx,
			)
		}
	}

	b.WriteString("\n")

	// Addresses
	b.WriteString(styles.HeaderText.Render("Addresses") + "\n")
	b.WriteString("──────────\n")
	if len(m.addresses) == 0 {
		b.WriteString("(none)\n")
	} else {
		fmt.Fprintf(&b, "%-10s  %-6s  %-16s  %-16s  %-16s\n",
			"IFACE", "PROTO", "ADDRESS", "NETMASK", "BROADCAST")
		for _, a := range m.addresses {
			fmt.Fprintf(&b, "%-10s  %-6s  %-16s  %-16s  %-16s\n",
				a.Iface, a.Proto,
				orDash(a.Address), orDash(a.Netmask), orDash(a.Broadcast),
			)
		}
	}

	b.WriteString("\n")

	// Open Ports
	b.WriteString(styles.HeaderText.Render("Open Ports") + "\n")
	b.WriteString("──────────\n")
	if len(m.ports) == 0 {
		b.WriteString("(none)\n")
	} else {
		fmt.Fprintf(&b, "%-6s  %-24s  %-12s  %-7s  %s\n",
			"PROTO", "LOCAL", "STATE", "PID", "PROCESS")
		for _, p := range m.ports {
			local := fmt.Sprintf("%s:%d", p.LocalIP, p.LocalPort)
			fmt.Fprintf(&b, "%-6s  %-24s  %-12s  %-7d  %s\n",
				p.Protocol, local, p.State, p.PID, p.Process,
			)
		}
	}

	return b.String()
}

// formatBytes formats a byte count as a human-readable string (K/M/G).
func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// truncate shortens s to max runes.
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
