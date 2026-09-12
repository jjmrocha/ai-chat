package theme

// Palette is the one palette, drawn from the terminal's own colours, for a user
// whose background this code cannot know. Two kinds of value appear here and
// neither is a fixed hex.
//
// An empty value means the terminal's default text colour. It is readable by
// construction — it is what the terminal already uses for ordinary text — so
// everything that must be read carries it.
//
// An ANSI index in 1-15 is a hue from the user's own profile. Only the
// decorative turn separator uses index 8: profiles routinely set it close to
// their background, which makes it the wrong choice for anything that carries
// words.
//
// Markdown replies are not styled from here at all — see ui.newRenderer.
var Palette = Theme{
	HeaderName: "1",
	User:       "1",
	Footer:     "",
	Error:      "9",
	Info:       "6",
	Activity:   "",
	TurnSep:    "8",
	Telemetry:  "",
}
