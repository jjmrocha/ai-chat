// Package theme holds the color palette the UI applies when rendering a chat.
// A Theme is plain data: the UI turns it into styles, so nothing below the UI
// depends on a styling library.
//
// There is one palette and it cannot be switched. A fixed set of colors can
// only be right for the background it was tuned against, and this code has no
// way to know the user's — so the palette defers to the terminal's own instead
// of picking for it.
package theme

// Theme is a set of colors, one per styled element of the chat UI. A color is
// an ANSI index from the terminal's palette, or empty for its default text
// color.
type Theme struct {
	HeaderName string
	User       string
	Footer     string
	Error      string
	Info       string
	Activity   string
	TurnSep    string
	Telemetry  string
}
