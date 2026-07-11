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
