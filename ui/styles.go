package ui

import (
	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
)

type styles struct {
	headerName lipgloss.Style
	user       lipgloss.Style
	info       lipgloss.Style
	err        lipgloss.Style
	activity   lipgloss.Style
	telemetry  lipgloss.Style
	turnSep    lipgloss.Style
	footer     lipgloss.Style
}

func fg(color string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}

func newStyles() styles {
	p := defaultPalette
	return styles{
		headerName: fg(p.HeaderName).Bold(true),
		user:       fg(p.User).Bold(true),
		info:       fg(p.Info),
		err:        fg(p.Error),
		activity:   fg(p.Activity).Italic(true),
		telemetry:  fg(p.Telemetry).Italic(true),
		turnSep:    fg(p.TurnSep),
		footer:     fg(p.Footer).Italic(true),
	}
}

func inputStyles() textarea.Styles {
	s := textarea.DefaultDarkStyles()
	for _, state := range []*textarea.StyleState{&s.Focused, &s.Blurred} {
		state.Base = lipgloss.NewStyle()
		state.CursorLine = lipgloss.NewStyle()
		state.Prompt = lipgloss.NewStyle()
		state.Text = lipgloss.NewStyle()
		state.Placeholder = fg(defaultPalette.TurnSep)
	}
	s.Cursor.Color = lipgloss.Color(defaultPalette.Info)
	return s
}
