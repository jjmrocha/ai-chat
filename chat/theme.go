package chat

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

var themes = map[string]Theme{
	"default": {
		HeaderName: "#FF9F1C",
		User:       "#FF9F1C",
		Footer:     "#94A3B8",
		Error:      "#F87171",
		Info:       "#00B4D8",
		Activity:   "#94A3B8",
		Rule:       "#4A5568",
		TurnSep:    "#4A5568",
		Telemetry:  "#94A3B8",
	},
	"monokai": {
		HeaderName: "#F92672",
		User:       "#F92672",
		Footer:     "#75715E",
		Error:      "#F92672",
		Info:       "#66D9EF",
		Activity:   "#75715E",
		Rule:       "#49483E",
		TurnSep:    "#49483E",
		Telemetry:  "#75715E",
	},
	"nord": {
		HeaderName: "#88C0D0",
		User:       "#88C0D0",
		Footer:     "#4C566A",
		Error:      "#BF616A",
		Info:       "#81A1C1",
		Activity:   "#4C566A",
		Rule:       "#3B4252",
		TurnSep:    "#3B4252",
		Telemetry:  "#4C566A",
	},
	"catppuccin": {
		HeaderName: "#F5C2E7",
		User:       "#F5C2E7",
		Footer:     "#6C7086",
		Error:      "#F38BA8",
		Info:       "#89B4FA",
		Activity:   "#6C7086",
		Rule:       "#45475A",
		TurnSep:    "#45475A",
		Telemetry:  "#6C7086",
	},
	"modern": {
		HeaderName: "#FF6B35",
		User:       "#FF6B35",
		Footer:     "#E2E8F0",
		Error:      "#F87171",
		Info:       "#00B4D8",
		Activity:   "#A7F3D0",
		Rule:       "#4A5568",
		TurnSep:    "#4A5568",
		Telemetry:  "#4A5568",
	},
}

func lookupTheme(name string) (Theme, bool) {
	t, ok := themes[name]
	return t, ok
}

func themeNames() []string {
	names := make([]string, 0, len(themes))
	for n := range themes {
		names = append(names, n)
	}
	return names
}