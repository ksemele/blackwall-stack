package safe

import "testing"

func TestLine(t *testing.T) {
	cases := map[string]string{
		"plain":            "plain",
		"a\tb":             "a b",
		"a\nb\rc":          "abc",
		"x\x1b[31mred":     "x[31mred",
		"del\x7fete":       "delete",
		"c1\u009bX\u0085y": "c1Xy",
		"Привет — 🙂":       "Привет — 🙂",
	}
	for in, want := range cases {
		if got := Line(in); got != want {
			t.Errorf("Line(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBlock(t *testing.T) {
	cases := map[string]string{
		"a\nb":          "a\nb",
		"a\r\nb\tc":     "a\nb c",
		"x\x1b]0;t\x07": "x]0;t",
		"Привет\n—🙂":    "Привет\n—🙂",
	}
	for in, want := range cases {
		if got := Block(in); got != want {
			t.Errorf("Block(%q) = %q, want %q", in, got, want)
		}
	}
}
