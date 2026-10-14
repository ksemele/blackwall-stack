package alerts

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/table"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

var cols = []table.Col{
	{Title: "TIME", Min: 25},
	{Title: "LVL", Min: 3},
	{Title: "AGENT", Min: 16},
	{Title: "RULE", Min: 7},
	{Title: "DESCRIPTION", Min: 20, Flex: true},
}

// visible returns the rows that pass the local text filter.
func (v *View) visible() []indexer.Alert { return v.filtered(v.rows) }

// filtered applies the local text filter to rows.
func (v *View) filtered(rows []indexer.Alert) []indexer.Alert {
	if v.filter == "" {
		return rows
	}
	f := strings.ToLower(v.filter)
	var out []indexer.Alert
	for _, a := range rows {
		hay := strings.ToLower(a.Rule.Description + " " + a.Agent.Name + " " + a.Rule.ID + " " + a.FullLog)
		if strings.Contains(hay, f) {
			out = append(out, a)
		}
	}
	return out
}

// fit cuts s to w display cells, ending with "…" when cut.
func fit(s string, w int) string {
	if w <= 0 || lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func levelStyle(l int) lipgloss.Style {
	switch {
	case l >= 12:
		return styles.LevelCritical
	case l >= 7:
		return styles.RowDown
	case l >= 5:
		return styles.RowPending
	}
	return lipgloss.NewStyle()
}

func (v *View) View() string {
	if v.card != nil {
		return v.card.view()
	}
	if v.searcher == nil {
		return fmt.Sprintf("indexer not configured for context %s — add an `indexer:` section (see tools/blackwall/README.md)\n", v.st.Context)
	}

	var b strings.Builder
	groups := "all groups"
	if v.excludeOn && len(v.st.ExcludeGroups) > 0 {
		groups = "hiding " + strings.Join(v.st.ExcludeGroups, ", ")
	}
	agent := "all"
	if v.agentID != "" {
		agent = v.agentName
	}
	rows := v.visible()
	left := fmt.Sprintf("level ≥ %d · %s · agent: %s · last 24h", v.minLevel, groups, agent)
	right := fmt.Sprintf("%d shown", len(rows))
	if v.newCount > 0 {
		right += fmt.Sprintf("  +%d new", v.newCount)
	}
	if v.paused {
		right += "  ⏸ paused"
	}
	pad := max(v.width-lipgloss.Width(left)-lipgloss.Width(right), 2)
	b.WriteString(fit(left+strings.Repeat(" ", pad)+right, v.width) + "\n")
	fixed := 2 // summary + table header

	if v.filtering || v.filter != "" {
		b.WriteString(fit("/"+v.filter, v.width) + "\n")
		fixed++
	}
	if v.err != "" {
		b.WriteString(styles.Error.Render(fit("⚠ indexer: "+v.err, v.width)) + "\n")
		fixed++
	}

	widths := table.Widths(cols, v.width)
	b.WriteString(fit(table.Header(cols, widths), v.width) + "\n")
	if len(rows) == 0 {
		if v.err == "" {
			b.WriteString("no alerts yet\n")
		}
		return b.String()
	}

	space := len(rows)
	if v.height > 0 {
		space = max(v.height-fixed, 1)
	}
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	if v.cursor >= v.offset+space {
		v.offset = v.cursor - space + 1
	}
	end := min(v.offset+space, len(rows))
	for i := v.offset; i < end; i++ {
		a := rows[i]
		row := fit(table.Row(widths, []string{
			wazuh.FormatLocalUTC(a.Time, "2006-01-02 15:04", "15:04Z"),
			fmt.Sprintf("%3d", a.Rule.Level),
			a.Agent.Name, a.Rule.ID, a.Rule.Description,
		}), v.width)
		if i == v.cursor {
			row = styles.SelectedRow.Render(row)
		} else {
			row = levelStyle(a.Rule.Level).Render(row)
		}
		b.WriteString(row + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}
