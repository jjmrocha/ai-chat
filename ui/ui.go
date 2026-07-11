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
)

const frameHeight = 3 // input + rule + status line

type refreshMsg struct{}

// observer bridges core notifications into the Bubble Tea event loop.
type observer struct{ program *tea.Program }

func (o *observer) TranscriptChanged() {
	if o.program != nil {
		o.program.Send(refreshMsg{})
	}
}

type model struct {
	core     *chat.Chat
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
		status := m.core.Name()
		if m.core.Busy() {
			status = "thinking…"
		}
		frame := lipgloss.JoinVertical(lipgloss.Left,
			m.input.View(),
			strings.Repeat("─", m.width),
			status,
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
		b.WriteString(ln.Text)
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
