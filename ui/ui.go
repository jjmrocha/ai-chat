// Package ui renders a chat.Chat as a Bubble Tea terminal program. It observes
// the core and re-renders on every transcript change; it holds no conversation
// state of its own.
package ui

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-chat/theme"
)

const frameHeight = 3 // input + rule + status line

type (
	refreshMsg struct{}
	quitMsg    struct{}
)

// observer bridges core notifications into the Bubble Tea event loop.
type observer struct{ program *tea.Program }

func (o *observer) TranscriptChanged() {
	if o.program != nil {
		o.program.Send(refreshMsg{})
	}
}

func (o *observer) Quit() {
	if o.program != nil {
		o.program.Send(quitMsg{})
	}
}

// styles holds the lipgloss styles derived from a theme, one per line Kind plus
// the footer. The core stores the theme as data; only the UI turns it into styles.
type styles struct {
	user      lipgloss.Style
	info      lipgloss.Style
	err       lipgloss.Style
	activity  lipgloss.Style
	telemetry lipgloss.Style
	footer    lipgloss.Style
}

func newStyles(t theme.Theme) styles {
	return styles{
		user:      lipgloss.NewStyle().Foreground(lipgloss.Color(t.User)).Bold(true),
		info:      lipgloss.NewStyle().Foreground(lipgloss.Color(t.Info)),
		err:       lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error)),
		activity:  lipgloss.NewStyle().Foreground(lipgloss.Color(t.Activity)).Italic(true),
		telemetry: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Telemetry)).Italic(true),
		footer:    lipgloss.NewStyle().Foreground(lipgloss.Color(t.Footer)).Italic(true),
	}
}

func (s styles) line(kind command.Kind, text string) string {
	switch kind {
	case command.User:
		return s.user.Render(text)
	case command.Info:
		return s.info.Render(text)
	case command.Error:
		return s.err.Render(text)
	case command.Activity:
		return s.activity.Render(text)
	case command.Telemetry:
		return s.telemetry.Render(text)
	default: // command.Reply
		return text
	}
}

type model struct {
	core     *chat.Chat
	styles   styles
	viewport viewport.Model
	input    textinput.Model
	width    int
	ready    bool
}

func newModel(core *chat.Chat) model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()
	return model{
		core:     core,
		styles:   newStyles(core.Theme()),
		viewport: viewport.New(),
		input:    ti,
	}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.viewport.SetWidth(msg.Width)
		m.viewport.SetHeight(max(msg.Height-frameHeight, 0))
		m.input.SetWidth(max(msg.Width-2, 0))
		m.ready = true
		return m.refresh(), nil

	case refreshMsg:
		return m.refresh(), nil

	case quitMsg:
		return m, tea.Quit

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			text := m.input.Value()
			m.input.Reset()
			m.core.Submit(text)
			return m, nil
		}
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m model) View() tea.View {
	content := "Initializing…"
	if m.ready {
		status := m.core.StatusText()
		if m.core.Busy() {
			status = "thinking…"
		}
		frame := lipgloss.JoinVertical(lipgloss.Left,
			m.input.View(),
			strings.Repeat("─", m.width),
			m.styles.footer.Render(status),
		)
		content = lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), frame)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeNone
	return v
}

func (m model) refresh() model {
	var b strings.Builder
	for i, ln := range m.core.Transcript() {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(m.styles.line(ln.Kind, ln.Text))
	}
	m.viewport.SetContent(b.String())
	m.viewport.GotoBottom()
	return m
}

// Run renders core in a Bubble Tea program until the user quits or ctx is done.
func Run(ctx context.Context, core *chat.Chat) error {
	p := tea.NewProgram(newModel(core), tea.WithContext(ctx))
	core.SetObserver(&observer{program: p})
	_, err := p.Run()
	return err
}
