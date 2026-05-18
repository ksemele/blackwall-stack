package agents

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// SecurityClient is the subset of wazuh.Client required by SecurityModel.
type SecurityClient interface {
	GetSyscheckLastScan(ctx context.Context, id string) (wazuh.SyscheckScan, error)
	ListSyscheckEntries(ctx context.Context, id string) ([]wazuh.SyscheckEntry, error)
	GetRootcheckLastScan(ctx context.Context, id string) (wazuh.RootcheckScan, error)
	ListRootcheckFindings(ctx context.Context, id string) ([]wazuh.RootcheckFinding, error)
	ListSCAPolicies(ctx context.Context, id string) ([]wazuh.SCAPolicy, error)
}

// SecurityLoadedMsg carries the result of a security fetch.
type SecurityLoadedMsg struct {
	FIMLastScan       wazuh.SyscheckScan
	FIMEntries        []wazuh.SyscheckEntry
	RootcheckLastScan wazuh.RootcheckScan
	RootcheckFindings []wazuh.RootcheckFinding
	SCAPolicies       []wazuh.SCAPolicy
}

// SecurityModel is the Security tab sub-view.
type SecurityModel struct {
	client      SecurityClient
	agentID     string
	fimScan     wazuh.SyscheckScan
	fimEntries  []wazuh.SyscheckEntry
	rcScan      wazuh.RootcheckScan
	rcFindings  []wazuh.RootcheckFinding
	scaPolicies []wazuh.SCAPolicy
	loaded      bool
}

func NewSecurity(c SecurityClient, agentID string) *SecurityModel {
	return &SecurityModel{client: c, agentID: agentID}
}

func (m *SecurityModel) Init() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		msg := SecurityLoadedMsg{}
		msg.FIMLastScan, _ = m.client.GetSyscheckLastScan(ctx, m.agentID)
		msg.FIMEntries, _ = m.client.ListSyscheckEntries(ctx, m.agentID)
		msg.RootcheckLastScan, _ = m.client.GetRootcheckLastScan(ctx, m.agentID)
		msg.RootcheckFindings, _ = m.client.ListRootcheckFindings(ctx, m.agentID)
		msg.SCAPolicies, _ = m.client.ListSCAPolicies(ctx, m.agentID)
		return msg
	}
}

func (m *SecurityModel) Update(msg tea.Msg) (subView, tea.Cmd) {
	if lm, ok := msg.(SecurityLoadedMsg); ok {
		m.fimScan = lm.FIMLastScan
		m.fimEntries = lm.FIMEntries
		m.rcScan = lm.RootcheckLastScan
		m.rcFindings = lm.RootcheckFindings
		m.scaPolicies = lm.SCAPolicies
		m.loaded = true
	}
	return m, nil
}

func (m *SecurityModel) View() string {
	if !m.loaded {
		return styles.Dim.Render("loading…") + "\n"
	}

	var b strings.Builder

	// FIM section
	b.WriteString(styles.HeaderText.Render("FIM (File Integrity Monitoring)") + "\n")
	b.WriteString("────────\n")
	if m.fimScan.Start == "" {
		b.WriteString("Last scan: (no scan yet)\n")
	} else {
		end := m.fimScan.End
		if end == "" {
			end = "—"
		}
		fmt.Fprintf(&b, "Last scan: %s → %s\n", m.fimScan.Start, end)
	}

	entries := m.fimEntries
	// sort by Mtime descending, fall back to Date
	sort.Slice(entries, func(i, j int) bool {
		ti := entries[i].Mtime
		if ti == "" {
			ti = entries[i].Date
		}
		tj := entries[j].Mtime
		if tj == "" {
			tj = entries[j].Date
		}
		return ti > tj
	})
	if len(entries) > 10 {
		entries = entries[:10]
	}

	if len(entries) == 0 {
		b.WriteString("Recent changes: (none)\n")
	} else {
		fmt.Fprintf(&b, "Recent changes: %d\n", len(m.fimEntries))
		for _, e := range entries {
			ts := e.Mtime
			if ts == "" {
				ts = e.Date
			}
			fmt.Fprintf(&b, "  • %-40s  %-10s  %s\n", e.File, e.Type, ts)
		}
	}

	b.WriteString("\n")

	// Rootcheck section
	b.WriteString(styles.HeaderText.Render("Rootcheck") + "\n")
	b.WriteString("────────\n")
	if m.rcScan.Start == "" {
		b.WriteString("Last scan: (no scan yet)\n")
	} else {
		end := m.rcScan.End
		if end == "" {
			end = "—"
		}
		fmt.Fprintf(&b, "Last scan: %s → %s\n", m.rcScan.Start, end)
	}

	var outstanding []wazuh.RootcheckFinding
	for _, f := range m.rcFindings {
		if f.Status != "solved" {
			outstanding = append(outstanding, f)
		}
	}

	if len(outstanding) == 0 {
		b.WriteString("Outstanding findings: (none)\n")
	} else {
		fmt.Fprintf(&b, "Outstanding findings: %d\n", len(outstanding))
		for _, f := range outstanding {
			label := styles.Disconnected.Render("[outstanding]")
			firstSeen := f.DateFirst
			if firstSeen == "" {
				firstSeen = "—"
			}
			fmt.Fprintf(&b, "  • %s %s  first seen %s\n", label, f.Log, firstSeen)
		}
	}

	b.WriteString("\n")

	// SCA section
	b.WriteString(styles.HeaderText.Render("SCA (Security Configuration Assessment)") + "\n")
	b.WriteString("────────\n")
	if len(m.scaPolicies) == 0 {
		b.WriteString("(no policies)\n")
	} else {
		for _, p := range m.scaPolicies {
			total := p.TotalChecks
			pass := p.Pass
			var pct int
			if total > 0 {
				pct = pass * 100 / total
			}
			scoreStr := fmt.Sprintf("pass %3d / %3d (%d%%)", pass, total, pct)
			lastScan := p.EndScan
			if lastScan == "" {
				lastScan = p.StartScan
			}
			if lastScan == "" {
				lastScan = "—"
			}
			passStyle := styles.Active
			if pct < 70 {
				passStyle = styles.Disconnected
			} else if pct < 85 {
				passStyle = styles.Pending
			}
			fmt.Fprintf(&b, "%-40s  %s   last scan %s\n",
				truncate(p.Name, 40),
				passStyle.Render(scoreStr),
				lastScan,
			)
		}
	}

	return b.String()
}
