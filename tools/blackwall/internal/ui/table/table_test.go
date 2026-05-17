package table

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestWidths_MinimumsPreservedWhenTooNarrow(t *testing.T) {
	cols := []Col{
		{Title: "A", Min: 4},
		{Title: "B", Min: 4, Flex: true},
	}
	got := Widths(cols, 4)
	want := []int{4, 4}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("widths[%d] = %d, want %d (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestWidths_TitleOverridesSmallerMin(t *testing.T) {
	cols := []Col{
		{Title: "LONGTITLE", Min: 3},
	}
	got := Widths(cols, 0)
	if got[0] != lipgloss.Width("LONGTITLE") {
		t.Errorf("widths[0] = %d, want %d", got[0], lipgloss.Width("LONGTITLE"))
	}
}

func TestWidths_DistributesExtraAcrossFlex(t *testing.T) {
	cols := []Col{
		{Title: "ID", Min: 4},
		{Title: "NAME", Min: 10, Flex: true},
		{Title: "LAST SEEN", Min: 10, Flex: true},
	}
	// totals: base=4+10+10+gap(2)*2 = 28; total=40 → extra=12 → 6 each flex
	got := Widths(cols, 40)
	if got[0] != 4 {
		t.Errorf("widths[0] (fixed) = %d, want 4", got[0])
	}
	if got[1] != 16 || got[2] != 16 {
		t.Errorf("flex widths = (%d, %d), want (16, 16)", got[1], got[2])
	}
}

func TestWidths_RemainderGoesToLeftmostFlex(t *testing.T) {
	cols := []Col{
		{Title: "A", Min: 4, Flex: true},
		{Title: "B", Min: 4, Flex: true},
	}
	// base = 4+4+gap(2)=10; total=15 → extra=5 → per=2, rem=1 → (4+3, 4+2)
	got := Widths(cols, 15)
	if got[0] != 7 || got[1] != 6 {
		t.Errorf("flex widths = (%d, %d), want (7, 6)", got[0], got[1])
	}
}

func TestRow_PadsTrailingColumnToFullWidth(t *testing.T) {
	widths := []int{4, 10}
	got := Row(widths, []string{"000", "—"})
	// 4 + gap(2) + 10 = 16
	if lipgloss.Width(got) != 16 {
		t.Errorf("row width = %d, want 16 (%q)", lipgloss.Width(got), got)
	}
	if !strings.HasPrefix(got, "000 ") {
		t.Errorf("row prefix = %q, want left-padded id", got)
	}
}

func TestRow_HandlesMissingValues(t *testing.T) {
	got := Row([]int{3, 3}, []string{"x"})
	if lipgloss.Width(got) != 8 { // 3 + gap(2) + 3
		t.Errorf("row width = %d, want 8 (%q)", lipgloss.Width(got), got)
	}
}

func TestHeader_UsesColumnTitles(t *testing.T) {
	cols := []Col{{Title: "ID", Min: 4}, {Title: "NAME", Min: 8, Flex: true}}
	widths := Widths(cols, 0)
	h := Header(cols, widths)
	if !strings.Contains(h, "ID") || !strings.Contains(h, "NAME") {
		t.Errorf("header missing titles: %q", h)
	}
}
