// Package theme holds the color palette the UI applies when rendering a chat.
// A Theme is plain data: the headless core stores the selected value and the UI
// turns it into styles, so the core never depends on a styling library.
package theme

import "strings"

// Theme is a set of hex colors, one per styled element of the chat UI.
type Theme struct {
	HeaderName string
	User       string
	Footer     string
	Error      string
	Info       string
	Activity   string
	Rule       string
	TurnSep    string
	Telemetry  string
}

// themes is the lookup of name → Theme used by ByName and Names.
var themes = map[string]Theme{
	"default":    Default,
	"nord":       Nord,
	"monokai":    Monokai,
	"catppuccin": Catppuccin,
}

// ByName returns the theme registered under name, or false if unknown.
func ByName(name string) (Theme, bool) {
	t, ok := themes[strings.ToLower(name)]
	return t, ok
}

// Names returns the sorted list of available theme names.
func Names() []string {
	out := make([]string, 0, len(themes))
	for n := range themes {
		out = append(out, n)
	}
	return out
}
