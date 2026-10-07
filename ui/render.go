package ui

import (
	"strings"

	"github.com/jjmrocha/ai-chat/chat"
)

const (
	userPrefix     = "❯ "
	activityPrefix = "● "
	detailPrefix   = "  ⎿ "
)

func (m model) renderBlock(ln chat.Line) string {
	s := m.styles
	switch ln.Kind {
	case chat.User:
		return s.user.Render(userPrefix + ln.Text)
	case chat.Info:
		return s.info.Render(ln.Text)
	case chat.Error:
		return s.err.Render(ln.Text)
	case chat.Activity:
		return m.renderActivity(ln)
	case chat.Telemetry:
		return s.turnSep.Render(m.printRule) + "\n" + s.telemetry.Render(ln.Text)
	case chat.Reply:
		return m.renderMarkdown(ln.Text)
	default:
		return ln.Text
	}
}

func (m model) renderActivity(ln chat.Line) string {
	head := m.styles.activity.Render(activityPrefix + ln.Text)
	if ln.Detail == "" {
		return head
	}

	return head + "\n" + m.styles.activity.Render(detailPrefix+ln.Detail)
}

func (m model) renderMarkdown(s string) string {
	if m.renderer == nil {
		return s
	}
	// glamour measures a tab as zero columns, so its padding overflows the line
	// once the terminal expands it; lipgloss spaces tabs the same way.
	s = strings.ReplaceAll(s, "\t", "    ")
	out, err := m.renderer.Render(s)
	if err != nil {
		return s
	}
	return strings.Trim(out, "\n")
}
