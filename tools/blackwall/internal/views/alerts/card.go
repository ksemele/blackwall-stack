package alerts

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/safe"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/scroll"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// card is the scrollable detail screen for one alert.
type card struct {
	alert indexer.Alert
	width int
	sl    scroll.List
}

func newCard(a indexer.Alert, width, height int) *card {
	c := &card{alert: a, sl: scroll.List{FooterRows: 1}}
	c.setSize(width, height)
	return c
}

func (c *card) setSize(width, height int) {
	c.width = width
	c.sl.SetSize(height)
}

func (c *card) handleKey(msg tea.KeyMsg) { c.sl.HandleKey(msg, len(c.lines())) }

func (c *card) hints() []footer.Hint {
	return []footer.Hint{{Key: "esc", Desc: "back"}, {Key: "j/k", Desc: "scroll"}, {Key: "g/G", Desc: "top/bottom"}}
}

func (c *card) view() string {
	lines := c.lines()
	return strings.TrimSuffix(c.sl.Render("", lines, c.sl.Indicator(len(lines))+"\n"), "\n")
}

// wrap splits s into chunks of at most w runes (w <= 0: no wrapping).
func wrap(s string, w int) []string {
	if w <= 0 {
		return []string{s}
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		r := []rune(para)
		for len(r) > w {
			out = append(out, string(r[:w]))
			r = r[w:]
		}
		out = append(out, string(r))
	}
	return out
}

// flatten turns nested data into "a.b.c" -> value pairs.
func flatten(prefix string, v any, out map[string]string) {
	if m, ok := v.(map[string]any); ok {
		for k, child := range m {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			flatten(key, child, out)
		}
		return
	}
	out[prefix] = fmt.Sprint(v)
}

func (c *card) lines() []string {
	a := c.alert
	w := c.width
	// Everything below comes from the indexer or from agents: clean it once.
	ln := safe.Line
	lns := func(in []string) string {
		out := make([]string, len(in))
		for i, x := range in {
			out[i] = ln(x)
		}
		return strings.Join(out, ", ")
	}
	var l []string
	add := func(s string) { l = append(l, wrap(s, w)...) }
	section := func(title string) {
		l = append(l, "", styles.HeaderText.Render(title), strings.Repeat("─", min(len(title), max(w, 1))))
	}

	// Wrap the plain title first, then style each line: wrapping a styled
	// string would cut ANSI escape sequences in half.
	for _, s := range wrap(fmt.Sprintf("[%d] %s", a.Rule.Level, ln(a.Rule.Description)), w) {
		l = append(l, levelStyle(a.Rule.Level).Render(s))
	}
	add(wazuh.FormatLocalUTC(a.Time, "2006-01-02 15:04:05 MST", "2006-01-02 15:04:05 UTC"))

	section("Rule")
	add(fmt.Sprintf("id %s · level %d · fired %d×", ln(a.Rule.ID), a.Rule.Level, a.Rule.FiredTimes))
	add("groups: " + lns(a.Rule.Groups))

	if m := a.Rule.MITRE; len(m.ID)+len(m.Tactic)+len(m.Technique) > 0 {
		section("MITRE ATT&CK")
		add("tactic:    " + lns(m.Tactic))
		add("technique: " + lns(m.Technique))
		add("id:        " + lns(m.ID))
	}

	section("Source")
	add(fmt.Sprintf("agent:    %s (%s)", ln(a.Agent.Name), ln(a.Agent.ID)))
	add("location: " + ln(a.Location))

	if len(a.Data) > 0 {
		section("Data")
		flat := map[string]string{}
		flatten("", a.Data, flat)
		keys := make([]string, 0, len(flat))
		for k := range flat {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			add(ln(k) + ": " + ln(flat[k]))
		}
	}

	if a.FullLog != "" {
		section("Full log")
		add(safe.Block(a.FullLog))
	}

	comp := []struct {
		name string
		vals []string
	}{{"PCI DSS", a.Rule.PCIDSS}, {"GDPR", a.Rule.GDPR}, {"NIST 800-53", a.Rule.NIST}, {"HIPAA", a.Rule.HIPAA}, {"TSC", a.Rule.TSC}}
	first := true
	for _, cv := range comp {
		if len(cv.vals) == 0 {
			continue
		}
		if first {
			section("Compliance")
			first = false
		}
		add(cv.name + ": " + lns(cv.vals))
	}
	return l
}
