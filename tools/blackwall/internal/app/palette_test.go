package app

import "testing"

func TestParsePalette_BareViewName(t *testing.T) {
	out, err := parsePalette(":agents", []string{":agents", ":contexts"})
	if err != nil {
		t.Fatalf("parsePalette: %v", err)
	}
	if sw, ok := out.(SwitchViewMsg); !ok || sw.Name != ":agents" {
		t.Errorf("got %#v, want SwitchViewMsg{:agents}", out)
	}
}

func TestParsePalette_CtxShortcut(t *testing.T) {
	out, err := parsePalette(":ctx prod-eu-1", []string{":agents", ":contexts"})
	if err != nil {
		t.Fatalf("parsePalette: %v", err)
	}
	cc, ok := out.(ContextChangedMsg)
	if !ok || cc.Name != "prod-eu-1" {
		t.Errorf("got %#v, want ContextChangedMsg{prod-eu-1}", out)
	}
}

func TestParsePalette_Quit(t *testing.T) {
	out, err := parsePalette(":q", nil)
	if err != nil {
		t.Fatalf("parsePalette: %v", err)
	}
	if _, ok := out.(QuitMsg); !ok {
		t.Errorf("got %#v, want QuitMsg", out)
	}
}

func TestParsePalette_Unknown(t *testing.T) {
	_, err := parsePalette(":nope", []string{":agents"})
	if err == nil {
		t.Error("expected error for unknown command")
	}
}
