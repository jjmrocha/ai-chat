// Package ui renders a chat.Chat as a Bubble Tea terminal program. It observes
// the core and re-renders on every transcript change; it holds no conversation
// state of its own beyond a cache of already-rendered lines.
package ui

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-chat/theme"
)

const (
	// frameHeight is the chrome around the input: title + rule + status line.
	// The input's own height is added on top, so it varies with content.
	frameHeight = 3
	// maxInputLines caps how far the input grows before it starts scrolling.
	maxInputLines = 6
)

type (
	refreshMsg struct{}
	quitMsg    struct{}
)

type observer struct{ program *tea.Program }

func (o *observer) TranscriptChanged() {
	if o.program != nil {
		go o.program.Send(refreshMsg{})
	}
}

func (o *observer) Quit() {
	if o.program != nil {
		go o.program.Send(quitMsg{})
	}
}

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
	input    textarea.Model
	spinner  spinner.Model
	renderer *glamour.TermRenderer
	width    int
	height   int
	ready    bool
	// mouseCapture routes wheel and drag events to the program. It stays off by
	// default: capturing them takes plain-drag text selection away from the
	// terminal, and copying matters more than scrolling by wheel.
	mouseCapture bool

	// rendered caches each transcript line's rendered form; lines are
	// append-only and immutable, so each is rendered (and markdown-parsed) once.
	// renderedWidth records the width they were rendered at.
	rendered      []string
	renderedWidth int

	// history holds submitted prompts, oldest first. histIdx points at the one
	// currently recalled; when it equals len(history) the user is editing their
	// own text rather than browsing, and draft is empty.
	history []string
	histIdx int
	draft   string

	lastTheme theme.Theme
}

func newModel(core chatCore) model {
	sty := newStyles(core.Theme())

	ti := textarea.New()
	ti.Placeholder = "Send a message…  (/help for commands)"
	ti.ShowLineNumbers = false
	ti.DynamicHeight = true
	ti.MinHeight = 1
	ti.MaxHeight = maxInputLines
	// Enter submits, so the newline moves to the modifiers terminals can send.
	ti.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("shift+enter", "alt+enter", "ctrl+j"),
		key.WithHelp("shift+enter", "insert newline"),
	)
	ti.SetPromptFunc(2, func(textarea.PromptInfo) string { return "❯ " })
	ti.Focus()
	tst := ti.Styles()
	tst.Focused.Prompt = sty.user
	ti.SetStyles(tst)

	vp := viewport.New()
	vp.KeyMap = pagerKeyMap()

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(core.Theme().Info))

	return model{
		core:          core,
		styles:        sty,
		lastTheme:     core.Theme(),
		viewport:      vp,
		input:         ti,
		spinner:       sp,
		renderer:      newRenderer(0),
		renderedWidth: -1,
	}
}

// pagerKeyMap keeps the transcript on paging keys only. The viewport sees every
// key the input does, so the stock keymap's bare letters (f, b, j, k, u, d,
// space) would scroll the transcript as the user types their message.
func pagerKeyMap() viewport.KeyMap {
	return viewport.KeyMap{
		PageDown:     key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
		PageUp:       key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "½ page down")),
		HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "½ page up")),
	}
}

func newRenderer(width int) *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	return r
}

func (m model) Init() tea.Cmd { return tea.Batch(textarea.Blink, m.spinner.Tick) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width != m.width {
			m.renderer = newRenderer(msg.Width)
		}
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.SetWidth(msg.Width)
		m.input.SetWidth(max(msg.Width-2, 0))
		m.layout()
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
			m.remember(text)
			m.core.Submit(text)
			m.layout()
			return m, nil
		case "f2":
			m.mouseCapture = !m.mouseCapture
			return m, nil
		case "up":
			if m.recallOlder() {
				return m, nil
			}
		case "down":
			if m.recallNewer() {
				return m, nil
			}
		}
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	m.layout()
	return m, tea.Batch(cmds...)
}

// remember appends a submitted prompt to the history and ends any browsing.
// Blank text and an immediate repeat of the newest entry are not recorded.
func (m *model) remember(text string) {
	m.draft = ""
	if trimmed := strings.TrimSpace(text); trimmed != "" {
		if len(m.history) == 0 || m.history[len(m.history)-1] != text {
			m.history = append(m.history, text)
		}
	}
	m.histIdx = len(m.history)
}

// recallOlder loads the previous prompt into the input, reporting whether it
// consumed the key. It declines while the cursor still has draft lines above
// it, so ↑ keeps moving the cursor inside a multi-line message.
func (m *model) recallOlder() bool {
	if m.input.Line() > 0 || m.histIdx == 0 {
		return false
	}
	if m.histIdx == len(m.history) {
		m.draft = m.input.Value()
	}
	m.histIdx--
	m.setInput(m.history[m.histIdx])
	return true
}

// recallNewer walks back toward the draft the user was typing, reporting
// whether it consumed the key.
func (m *model) recallNewer() bool {
	if m.histIdx >= len(m.history) || m.input.Line() < m.input.LineCount()-1 {
		return false
	}
	m.histIdx++
	if m.histIdx == len(m.history) {
		m.setInput(m.draft)
		m.draft = ""
		return true
	}
	m.setInput(m.history[m.histIdx])
	return true
}

func (m *model) setInput(text string) {
	m.input.SetValue(text)
	m.input.CursorEnd()
	m.layout()
}

// layout gives the viewport whatever height the grown input leaves behind.
func (m *model) layout() {
	m.viewport.SetHeight(max(m.height-frameHeight-m.input.Height(), 0))
}

func (m model) View() tea.View {
	content := "Initializing…"
	if m.ready {
		status := m.styles.footer.Render(m.core.StatusText())
		if m.core.Busy() {
			status = m.spinner.View() + m.styles.footer.Render(" thinking…")
		}
		content = lipgloss.JoinVertical(lipgloss.Left,
			m.viewport.View(),
			m.titleBar(),
			m.input.View(),
			m.styles.headerName.Render(m.hrule()),
			status,
		)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	if m.mouseCapture {
		v.MouseMode = tea.MouseModeCellMotion
	} else {
		v.MouseMode = tea.MouseModeNone
	}
	return v
}

func (m model) hrule() string { return strings.Repeat("─", m.width) }

func (m model) titleBar() string {
	name := m.core.Name()
	if name == "" {
		return m.styles.headerName.Render(m.hrule())
	}
	if m.mouseCapture {
		name += " · mouse"
	}
	label := " " + name + " "
	left := 5
	right := max(0, m.width-left-lipgloss.Width(label))
	l := m.styles.headerName.Render(strings.Repeat("─", left))
	r := m.styles.headerName.Render(strings.Repeat("─", right))
	return l + m.styles.headerName.Render(label) + r
}

func (m model) refresh() model {
	t := m.core.Theme()
	if m.lastTheme != t {
		m.styles = newStyles(t)
		m.lastTheme = t
		m.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Info))
		tst := m.input.Styles()
		tst.Focused.Prompt = m.styles.user
		m.input.SetStyles(tst)
		m.rendered = m.rendered[:0]
	}

	lines := m.core.Transcript()

	if m.renderedWidth != m.width || len(lines) < len(m.rendered) {
		m.rendered = m.rendered[:0]
		m.renderedWidth = m.width
	}
	for i := len(m.rendered); i < len(lines); i++ {
		m.rendered = append(m.rendered, m.renderBlock(lines[i]))
	}

	if len(m.rendered) == 0 {
		m.viewport.SetContent("")
	} else {
		m.viewport.SetContent(strings.Join(m.rendered, "\n\n"))
	}
	m.viewport.GotoBottom()
	return m
}

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
	core.SetContext(ctx)
	p := tea.NewProgram(newModel(core), tea.WithContext(ctx))
	core.SetObserver(&observer{program: p})
	_, err := p.Run()
	return err
}
