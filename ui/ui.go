// Package ui renders a chat.Chat as an inline Bubble Tea terminal program. It
// observes the core and prints each new transcript line above a live region
// holding the input and the status bar. Finished lines belong to the terminal's
// scrollback from then on, so selection, copying and wheel scrolling stay
// native; the program never enters the alternate screen or captures the mouse.
package ui

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-chat/theme"
)

const (
	// maxInputLines caps how far the input grows before it starts scrolling.
	maxInputLines = 6
	// inputIndent pads every row of the input, standing in for the prompt marker
	// the input no longer carries and lining it up with the indented replies.
	inputIndent = "  "
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
	input    textarea.Model
	spinner  spinner.Model
	renderer *glamour.TermRenderer
	width    int
	ready    bool

	// printed counts the transcript lines already sent to the scrollback. The
	// transcript is append-only apart from /clear, so this doubles as the mark
	// separating printed history from what still has to go out.
	printed int

	// quitting blanks the live region for the final render, so the shell prompt
	// comes back under the conversation instead of under a stale input box.
	quitting bool

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
	ti.Prompt = inputIndent
	ti.Focus()

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(core.Theme().Info))

	return model{
		core:      core,
		styles:    sty,
		lastTheme: core.Theme(),
		input:     ti,
		spinner:   sp,
		renderer:  newRenderer(0),
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
		m.input.SetWidth(max(msg.Width-2, 0))
		m.ready = true
		return m.emit()

	case refreshMsg:
		return m.emit()

	case quitMsg:
		m.quitting = true
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			text := m.input.Value()
			m.input.Reset()
			m.remember(text)
			m.core.Submit(text)
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

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
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
}

func (m model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	content := "Initializing…"
	if m.ready {
		status := m.styles.footer.Render(m.core.StatusText())
		if m.core.Busy() {
			status = m.spinner.View() + m.styles.footer.Render(" thinking…")
		}
		content = lipgloss.JoinVertical(lipgloss.Left,
			m.titleBar(),
			m.input.View(),
			m.styles.headerName.Render(m.hrule()),
			status,
		)
	}

	v := tea.NewView(content)
	v.AltScreen = false
	v.MouseMode = tea.MouseModeNone
	return v
}

func (m model) hrule() string { return strings.Repeat("─", m.width) }

// titleBar is the rule that caps the live region, carrying the chat's name.
func (m model) titleBar() string {
	name := m.core.Name()
	if name == "" {
		return m.styles.headerName.Render(m.hrule())
	}
	label := " " + name + " "
	left := 5
	right := max(0, m.width-left-lipgloss.Width(label))
	l := m.styles.headerName.Render(strings.Repeat("─", left))
	r := m.styles.headerName.Render(strings.Repeat("─", right))
	return l + m.styles.headerName.Render(label) + r
}

// emit sends every transcript line not yet printed to the scrollback and, when
// the transcript has shrunk under it, wipes the screen first.
func (m model) emit() (tea.Model, tea.Cmd) {
	m.restyle()

	// A shrunk transcript means /clear reset the session. The conversation stays
	// in the scrollback where the user can still read it; only the mark moves,
	// so whatever follows the reset is printed from the start.
	if len(m.core.Transcript()) < m.printed {
		m.printed = 0
	}

	blocks := m.pending()
	if len(blocks) == 0 {
		return m, nil
	}
	m.printed += len(blocks)

	cmds := make([]tea.Cmd, 0, len(blocks))
	for _, b := range blocks {
		cmds = append(cmds, tea.Println(b))
	}
	return m, tea.Sequence(cmds...)
}

// pending renders the transcript lines that have not reached the terminal yet.
func (m model) pending() []string {
	lines := m.core.Transcript()
	if len(lines) <= m.printed {
		return nil
	}
	blocks := make([]string, 0, len(lines)-m.printed)
	for _, ln := range lines[m.printed:] {
		// A blank line on each side gives every block room of its own.
		blocks = append(blocks, "\n"+m.renderBlock(ln)+"\n")
	}
	return blocks
}

// restyle repoints the styles at the active theme. Lines already in the
// scrollback keep the palette they were printed with; only new ones change.
func (m *model) restyle() {
	t := m.core.Theme()
	if m.lastTheme == t {
		return
	}
	m.styles = newStyles(t)
	m.lastTheme = t
	m.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Info))
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
	return strings.Trim(out, "\n")
}

// Run renders core in a Bubble Tea program until the user quits or ctx is done.
func Run(ctx context.Context, core *chat.Chat) error {
	core.SetContext(ctx)
	p := tea.NewProgram(newModel(core), tea.WithContext(ctx))
	core.SetObserver(&observer{program: p})
	_, err := p.Run()
	return err
}
