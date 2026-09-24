package ui

import (
	"strings"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/command"
)

const (
	userPrefix     = "❯ "
	activityPrefix = "● "
	detailPrefix   = "  ⎿ "
)

func (m model) renderBlock(ln chat.Line) string {
	s := m.styles
	switch ln.Kind {
	case command.User:
		return s.user.Render(userPrefix + ln.Text)
	case command.Info:
		return s.info.Render(ln.Text)
	case command.Error:
		return s.err.Render(ln.Text)
	case command.Activity:
		return m.renderActivity(ln)
	case command.Telemetry:
		return s.turnSep.Render(m.printRule) + "\n" + s.telemetry.Render(ln.Text)
	case command.Reply:
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

	return head + "\n" + m.styles.telemetry.Render(detailPrefix+ln.Detail)
}

func (m model) renderMarkdown(s string) string {
	if m.renderer == nil {
		return s
	}
	out, err := m.renderer.Render(s)
	if err != nil {
		return s
	}
	return strings.Trim(out, "\n")
}
