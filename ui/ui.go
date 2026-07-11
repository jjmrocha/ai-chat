// Package ui renders a chat.Chat as a Bubble Tea terminal program. It observes
// the core and re-renders on every transcript change; it holds no conversation
// state of its own beyond a cache of already-rendered lines.
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

const frameHeight = 4 // title + input + rule + status line

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
	fg := func(hex string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
	}
	return styles{
		headerName: fg(t.HeaderName).Bold(true),
		user:       fg(t.User).Bold(true),
		info:       fg(t.Info),
		err:        fg(t.Error),
		activity:   fg(t.Activity).Italic(true),
		telemetry:  fg(t.Telemetry).Italic(true),
		rule:       fg(t.Rule),
		turnSep:    fg(t.TurnSep),
		footer:     fg(t.Footer).Italic(true),
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

	// rendered caches each transcript line's rendered form; lines are
	// append-only and immutable, so each is rendered (and markdown-parsed) once.
	// renderedWidth records the width they were rendered at.
	rendered      []string
	renderedWidth int
}

func newModel(core chatCore) model {
	sty := newStyles(core.Theme())

	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.Placeholder = "Send a message…  (/help for commands)"
	ti.Focus()
	tst := ti.Styles()
	tst.Focused.Prompt = sty.user
	ti.SetStyles(tst)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(core.Theme().Info))

	return model{
		core:          core,
		styles:        sty,
		viewport:      viewport.New(),
		input:         ti,
		spinner:       sp,
		renderer:      newRenderer(0),
		renderedWidth: -1,
	}
}

func newRenderer(width int) *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	return r
}

func (m model) Init() tea.Cmd { return tea.Batch(textinput.Blink, m.spinner.Tick) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width != m.width {
			m.renderer = newRenderer(msg.Width)
		}
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
		status := m.styles.footer.Render(m.core.StatusText())
		if m.core.Busy() {
			status = m.spinner.View() + m.styles.footer.Render(" thinking…")
		}
		content = lipgloss.JoinVertical(lipgloss.Left,
			m.titleBar(),
			m.viewport.View(),
			m.input.View(),
			m.styles.rule.Render(m.hrule()),
			status,
		)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeNone
	return v
}

func (m model) hrule() string { return strings.Repeat("─", m.width) }

// titleBar renders the chat name inset in a horizontal rule at the top.
func (m model) titleBar() string {
	name := m.core.Name()
	if name == "" {
		return m.styles.rule.Render(m.hrule())
	}
	label := " " + name + " "
	side := max(0, m.width-lipgloss.Width(label))
	left := m.styles.rule.Render(strings.Repeat("─", side/2))
	right := m.styles.rule.Render(strings.Repeat("─", side-side/2))
	return left + m.styles.headerName.Render(label) + right
}

func (m model) refresh() model {
	lines := m.core.Transcript()

	// Rebuild the cache from scratch on a width change or a shrink (e.g. /clear);
	// otherwise render only the newly-appended lines.
	if m.renderedWidth != m.width || len(lines) < len(m.rendered) {
		m.rendered = m.rendered[:0]
		m.renderedWidth = m.width
	}
	for i := len(m.rendered); i < len(lines); i++ {
		m.rendered = append(m.rendered, m.renderBlock(lines[i]))
	}

	if len(m.rendered) == 0 {
		m.viewport.SetContent(m.welcome())
	} else {
		m.viewport.SetContent(strings.Join(m.rendered, "\n\n"))
	}
	m.viewport.GotoBottom()
	return m
}

// welcome is the empty-state shown before the first message.
func (m model) welcome() string {
	name := m.core.Name()
	if name == "" {
		name = "Chat"
	}
	return "\n" + m.styles.headerName.Render(name) + "\n\n" +
		m.styles.footer.Render("Send a message and press Enter · /help for commands · Ctrl+C to quit")
}

// renderBlock styles one transcript line by its Kind: replies as markdown, a
// telemetry line under a turn separator, everything else as a themed line.
func (m model) renderBlock(ln chat.Line) string {
	s := m.styles
	switch ln.Kind {
	case command.User:
		return s.user.Render(ln.Text)
	case command.Info:
		return s.info.Render(ln.Text)
	case command.Error:
		return s.err.Render(ln.Text)
	case command.Activity:
		return s.activity.Render(ln.Text)
	case command.Telemetry:
		return s.turnSep.Render(m.hrule()) + "\n" + s.telemetry.Render(ln.Text)
	case command.Reply:
		return m.renderMarkdown(ln.Text)
	default:
		return ln.Text
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
