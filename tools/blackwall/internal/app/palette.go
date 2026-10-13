package app

import (
	"fmt"
	"strings"
)

// QuitMsg requests app shutdown via the palette (`:q`).
type QuitMsg struct{}

// parsePalette turns a user-typed palette string into an internal message.
// `knownViews` is the list of registered top-level view names (each starts with `:`).
func parsePalette(input string, knownViews []string) (any, error) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, ":") {
		input = ":" + input
	}

	if input == ":q" || input == ":quit" {
		return QuitMsg{}, nil
	}

	// `:ctx <name>` switches contexts.
	if strings.HasPrefix(input, ":ctx ") {
		name := strings.TrimSpace(strings.TrimPrefix(input, ":ctx "))
		if name == "" {
			return nil, fmt.Errorf(":ctx requires a name")
		}
		return ContextChangedMsg{Name: name}, nil
	}

	// Bare view name (`:agents`, `:contexts`).
	for _, v := range knownViews {
		if v == input {
			return SwitchViewMsg{Name: input}, nil
		}
	}

	return nil, fmt.Errorf("unknown command: %s", input)
}
