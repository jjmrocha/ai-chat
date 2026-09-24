package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jjmrocha/ai-chat/internal/format"
)

const (
	idlePlaceholder   = "Send a message…  (/help for commands)"
	queuedPlaceholder = "(queued)"
)

func (m model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	content := "Initializing…"
	if m.ready {
		m.input.Placeholder = m.placeholder()
		content = m.liveRegion()
	}

	v := tea.NewView(content)
	v.AltScreen = false
	v.MouseMode = tea.MouseModeNone
	return v
}

func (m model) liveRegion() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		m.thinkingLine(),
		m.titleBar,
		m.input.View(),
		m.styles.headerName.Render(m.hrule),
		m.styles.footer.Render(m.core.StatusText()),
	)
}

func (m *model) resize() {
	m.hrule = strings.Repeat("─", m.width)
	m.printRule = strings.Repeat("─", m.printWidth())
	m.titleBar = m.renderTitleBar()
}

func (m model) renderTitleBar() string {
	name := m.core.Name()
	if name == "" {
		return m.styles.headerName.Render(m.hrule)
	}
	label := " " + name + " "
	left := 5
	right := max(0, m.width-left-lipgloss.Width(label))
	l := m.styles.headerName.Render(strings.Repeat("─", left))
	r := m.styles.headerName.Render(strings.Repeat("─", right))
	return l + m.styles.headerName.Render(label) + r
}

func (m model) thinkingLine() string {
	if m.thinkingSince.IsZero() {
		return ""
	}
	return m.spinner.View() + m.styles.footer.Render(" "+m.thinkingLabel())
}

func (m model) thinkingLabel() string {
	if m.core.Cancelling() {
		return "Cancelling…"
	}

	elapsed := format.Duration(time.Since(m.thinkingSince))
	if tool := m.core.PendingTool(); tool != "" {
		return tool + " · " + elapsed
	}
	if cmd := m.core.PendingCommand(); cmd != "" {
		return "Waiting for " + cmd + " · " + elapsed
	}
	return "Thinking for " + elapsed
}

func (m model) placeholder() string {
	if m.core.Queued() {
		return queuedPlaceholder
	}
	return idlePlaceholder
}
