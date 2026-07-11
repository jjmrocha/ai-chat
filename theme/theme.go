// Package theme holds the color palette the UI applies when rendering a chat.
// A Theme is plain data: the headless core stores the selected value and the UI
// turns it into styles, so the core never depends on a styling library.
package theme

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

// Default is the palette used when no theme is supplied.
var Default = Theme{
	HeaderName: "#FF9F1C",
	User:       "#FF9F1C",
	Footer:     "#94A3B8",
	Error:      "#F87171",
	Info:       "#00B4D8",
	Activity:   "#94A3B8",
	Rule:       "#4A5568",
	TurnSep:    "#4A5568",
	Telemetry:  "#94A3B8",
}

// Monokai is the classic Monokai palette.
var Monokai = Theme{
	HeaderName: "#F92672",
	User:       "#F92672",
	Footer:     "#75715E",
	Error:      "#F92672",
	Info:       "#66D9EF",
	Activity:   "#75715E",
	Rule:       "#49483E",
	TurnSep:    "#49483E",
	Telemetry:  "#75715E",
}

// Nord is the Nord palette.
var Nord = Theme{
	HeaderName: "#88C0D0",
	User:       "#88C0D0",
	Footer:     "#4C566A",
	Error:      "#BF616A",
	Info:       "#81A1C1",
	Activity:   "#4C566A",
	Rule:       "#3B4252",
	TurnSep:    "#3B4252",
	Telemetry:  "#4C566A",
}

// Catppuccin is the Catppuccin (Mocha) palette.
var Catppuccin = Theme{
	HeaderName: "#F5C2E7",
	User:       "#F5C2E7",
	Footer:     "#6C7086",
	Error:      "#F38BA8",
	Info:       "#89B4FA",
	Activity:   "#6C7086",
	Rule:       "#45475A",
	TurnSep:    "#45475A",
	Telemetry:  "#6C7086",
}

// Modern is a high-contrast palette with warm accents.
var Modern = Theme{
	HeaderName: "#FF6B35",
	User:       "#FF6B35",
	Footer:     "#E2E8F0",
	Error:      "#F87171",
	Info:       "#00B4D8",
	Activity:   "#A7F3D0",
	Rule:       "#4A5568",
	TurnSep:    "#4A5568",
	Telemetry:  "#4A5568",
}
