// Package ui renders a chat.Chat as a Bubble Tea terminal program. It observes
// the core and re-renders on every transcript change; it holds no conversation
// state of its own.
package ui

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-chat/theme"
)

const frameHeight = 4 // header + input + rule + status line

type (
	refreshMsg struct{}
	quitMsg    struct{}
)

// observer bridges core notifications into the Bubble Tea event loop. These two
// signals — re-render and quit — are the only inbound channel from the core.
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

// styles holds the lipgloss styles derived from a theme. The core stores the
// theme as data; only the UI turns it into styles.
type styles struct {
	headerName lipgloss.Style
	user       lipgloss.Style
	info       lipgloss.Style
	err        lipgloss.Style
	activity   lipgloss.Style
	telemetry  lipgloss.Style
	rule       lipgloss.Style
	turnSep    lipgloss.Style
	footer     lipgloss.Style
}

func newStyles(t theme.Theme) styles {
	return styles{
		headerName: lipgloss.NewStyle().Foreground(lipgloss.Color(t.HeaderName)).Bold(true),
		user:       lipgloss.NewStyle().Foreground(lipgloss.Color(t.User)).Bold(true),
		info:       lipgloss.NewStyle().Foreground(lipgloss.Color(t.Info)),
		err:        lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error)),
		activity:   lipgloss.NewStyle().Foreground(lipgloss.Color(t.Activity)).Italic(true),
		telemetry:  lipgloss.NewStyle().Foreground(lipgloss.Color(t.Telemetry)).Italic(true),
		rule:       lipgloss.NewStyle().Foreground(lipgloss.Color(t.Rule)),
		turnSep:    lipgloss.NewStyle().Foreground(lipgloss.Color(t.TurnSep)),
		footer:     lipgloss.NewStyle().Foreground(lipgloss.Color(t.Footer)).Italic(true),
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

// chatCore is the slice of *chat.Chat the UI observes and renders. Kept as an
// interface so the view can be rendered in tests without a live agent.
type chatCore interface {
	Name() string
	Theme() theme.Theme
	Transcript() []chat.Line
	Busy() bool
	StatusText() string
	Submit(text string)
}

type model struct {
	core     chatCore
	styles   styles
	viewport viewport.Model
	input    textinput.Model
	spinner  spinner.Model
	renderer *glamour.TermRenderer
	width    int
	ready    bool
}

func newModel(core chatCore) model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()
	ti.Placeholder = "type /help for commands"

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(0),
	)

	return model{
		core:     core,
		styles:   newStyles(core.Theme()),
		viewport: viewport.New(),
		input:    ti,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
		renderer: renderer,
	}
}

func (m model) Init() tea.Cmd { return tea.Batch(textinput.Blink, m.spinner.Tick) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.viewport.SetWidth(msg.Width)
		m.viewport.SetHeight(max(msg.Height-frameHeight, 0))
		m.input.SetWidth(max(msg.Width-2, 0))
		if r, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(msg.Width),
		); err == nil {
			m.renderer = r
		}
		m.ready = true
		return m.refresh(), nil

	case refreshMsg:
		return m.refresh(), nil

	case quitMsg:
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

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
			status = m.spinner.View() + " thinking…"
		}
		frame := lipgloss.JoinVertical(lipgloss.Left,
			m.header(),
			m.input.View(),
			m.styles.rule.Render(strings.Repeat("─", m.width)),
			m.styles.footer.Render(status),
		)
		content = lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), frame)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeNone
	return v
}

// header renders the chat name inset in a horizontal rule.
func (m model) header() string {
	name := m.core.Name()
	if name == "" {
		return m.styles.rule.Render(strings.Repeat("─", m.width))
	}
	label := " " + name + " "
	bar := strings.Repeat("─", max(0, m.width-lipgloss.Width(label))/2)
	return m.styles.headerName.Render(bar + label + bar)
}

func (m model) refresh() model {
	var b strings.Builder
	for i, ln := range m.core.Transcript() {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(m.renderBlock(ln))
	}
	m.viewport.SetContent(b.String())
	m.viewport.GotoBottom()
	return m
}

// renderBlock styles a transcript line. Replies are rendered as markdown; a
// telemetry line is preceded by a turn separator.
func (m model) renderBlock(ln chat.Line) string {
	switch ln.Kind {
	case command.Reply:
		return m.renderMarkdown(ln.Text)
	case command.Telemetry:
		sep := m.styles.turnSep.Render(strings.Repeat("━", m.width))
		return sep + "\n" + m.styles.telemetry.Render(ln.Text)
	default:
		return m.styles.line(ln.Kind, ln.Text)
	}
}

func (m model) renderMarkdown(s string) string {
	if m.renderer == nil {
		return s
	}
	out, err := m.renderer.Render(s)
	if err != nil {
		return s
	}
	return strings.TrimRight(out, "\n")
}

// Run renders core in a Bubble Tea program until the user quits or ctx is done.
func Run(ctx context.Context, core *chat.Chat) error {
	p := tea.NewProgram(newModel(core), tea.WithContext(ctx))
	core.SetObserver(&observer{program: p})
	_, err := p.Run()
	return err
}
