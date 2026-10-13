package agents

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// HardwareClient is the subset of wazuh.Client required by HardwareModel.
type HardwareClient interface {
	GetHardware(ctx context.Context, id string) (wazuh.Hardware, error)
	GetOSInfo(ctx context.Context, id string) (wazuh.OSInfo, error)
}

// HardwareLoadedMsg carries the result of a hardware+OS fetch.
type HardwareLoadedMsg struct {
	HW  wazuh.Hardware
	OS  wazuh.OSInfo
	Err error
}

// HardwareModel is the Hardware tab sub-view.
type HardwareModel struct {
	client  HardwareClient
	agentID string
	hw      wazuh.Hardware
	osInfo  wazuh.OSInfo
	loaded  bool
	err     string
}

func NewHardware(c HardwareClient, agentID string) *HardwareModel {
	return &HardwareModel{client: c, agentID: agentID}
}

func (m *HardwareModel) Init() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		hw, err := m.client.GetHardware(ctx, m.agentID)
		if err != nil {
			return HardwareLoadedMsg{Err: err}
		}
		os, err := m.client.GetOSInfo(ctx, m.agentID)
		if err != nil {
			return HardwareLoadedMsg{Err: err}
		}
		return HardwareLoadedMsg{HW: hw, OS: os}
	}
}

func (m *HardwareModel) Update(msg tea.Msg) (subView, tea.Cmd) {
	if lm, ok := msg.(HardwareLoadedMsg); ok {
		if lm.Err != nil {
			m.err = lm.Err.Error()
			return m, nil
		}
		m.hw = lm.HW
		m.osInfo = lm.OS
		m.loaded = true
		m.err = ""
	}
	return m, nil
}

func (m *HardwareModel) View() string {
	if m.err != "" {
		return styles.Error.Render("⚠ "+m.err) + "\n"
	}
	if !m.loaded {
		return styles.Dim.Render("loading…") + "\n"
	}

	var b strings.Builder

	b.WriteString(styles.HeaderText.Render("Hardware") + "\n")
	b.WriteString("────────\n")
	fmt.Fprintf(&b, "CPU:       %s (%d cores @ %.0f MHz)\n",
		m.hw.CPUName, m.hw.CPUCores, m.hw.CPUMHz)
	ramTotalMB := m.hw.RAMTotal / 1024
	ramFreeMB := m.hw.RAMFree / 1024
	fmt.Fprintf(&b, "RAM:       %d MB total, %d MB free (%.0f%%)\n",
		ramTotalMB, ramFreeMB, m.hw.RAMUsage)
	fmt.Fprintf(&b, "Board:     %s\n", orDash(m.hw.BoardSerial))
	fmt.Fprintf(&b, "Scanned:   %s\n", orDash(m.hw.ScanTime))

	b.WriteString("\n")
	b.WriteString(styles.HeaderText.Render("OS") + "\n")
	b.WriteString("────────\n")
	distro := strings.TrimSpace(m.osInfo.OSName + " " + m.osInfo.OSVersion)
	if m.osInfo.OSCodename != "" {
		distro += " (" + m.osInfo.OSCodename + ")"
	}
	fmt.Fprintf(&b, "Distro:    %s\n", orDash(distro))
	fmt.Fprintf(&b, "Hostname:  %s\n", orDash(m.osInfo.Hostname))
	fmt.Fprintf(&b, "Kernel:    %s\n", orDash(m.osInfo.Release))
	fmt.Fprintf(&b, "Arch:      %s\n", orDash(m.osInfo.Architecture))
	fmt.Fprintf(&b, "Platform:  %s\n", orDash(m.osInfo.OSPlatform))

	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
