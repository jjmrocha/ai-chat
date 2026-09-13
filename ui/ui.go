// Package ui is a Bubble Tea front-end for a chat.Chat.
//
// [Run] is the whole public surface: hand it a core and it takes over the
// terminal until the user quits. It is one Observer of the core, not part of
// it — replace this package with your own renderer without touching your agent
// logic.
//
// The UI renders inline rather than taking over the screen. Finished transcript
// lines are printed above a small live region holding the thinking row, title
// bar, input and status line, so the conversation lands in the terminal's own
// scrollback and selection, copying and scrolling stay the terminal's. The
// trade-off is that printed lines are never repainted, so resizing leaves
// earlier markdown wrapped at the old width.
//
// Keys: Enter sends, Shift+Enter (or Alt+Enter, Ctrl+J) inserts a newline, Up
// and Down walk prompt history, Ctrl+C quits.
//
// Colors come from one fixed palette that cannot be switched. Every value is
// either an ANSI index, which the terminal's own profile defines, or empty,
// meaning the terminal's default text color — a fixed set of hex colors can
// only be right for the background it was tuned against, and neither this
// package nor the program embedding it can know the user's. Markdown replies
// follow the same rule: glamour's dark layout with every color taken from the
// palette, and fenced code highlighted in the 16 basic ANSI colors.
package ui

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/command"
)

const (
	maxInputLines = 6

	inputIndent = "  "

	idlePlaceholder   = "Send a message…  (/help for commands)"
	queuedPlaceholder = "(queued)"

	userPrefix     = "❯ "
	activityPrefix = "● "
	detailPrefix   = "  ⎿ "
)

type (
	refreshMsg struct{}
	quitMsg    struct{}

	printedMsg struct{}
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

type chatCore interface {
	Name() string
	TranscriptLen() int
	Since(n int) []chat.Line
	Busy() bool
	StatusText() string
	Queued() bool
	PendingTool() string
	Submit(text string)
}

type model struct {
	core     chatCore
	styles   styles
	input    textarea.Model
	spinner  spinner.Model
	renderer *glamour.TermRenderer
	width    int
	height   int
	ready    bool

	printed       int
	thinkingSince time.Time
	printing      bool
	quitting      bool
	spinning      bool

	hrule    string
	titleBar string

	history []string
	histIdx int
	draft   string
}

func newModel(core chatCore) model {
	sty := newStyles()

	ti := textarea.New()
	ti.ShowLineNumbers = false
	ti.DynamicHeight = true
	ti.MinHeight = 1
	ti.MaxHeight = maxInputLines

	ti.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("shift+enter", "alt+enter", "ctrl+j"),
		key.WithHelp("shift+enter", "insert newline"),
	)
	ti.Prompt = inputIndent
	ti.SetStyles(inputStyles())
	ti.Focus()

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = fg(defaultPalette.Info)

	return model{
		core:     core,
		styles:   sty,
		input:    ti,
		spinner:  sp,
		renderer: newRenderer(0),
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

func newRenderer(width int) *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(markdownStyle(defaultPalette)),
		glamour.WithChromaFormatter("terminal16"),
		glamour.WithWordWrap(width),
	)
	return r
}

func (m model) Init() tea.Cmd { return textarea.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	tick := m.trackThinking()

	next, cmd := m.update(msg)
	if tick == nil {
		return next, cmd
	}
	return next, tea.Batch(cmd, tick)
}

func (m model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width != m.width {
			m.renderer = newRenderer(msg.Width)
		}
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(max(msg.Width-2, 0))
		m.resize()
		m.ready = true
		return m.emit()

	case refreshMsg:
		return m.emit()

	case printedMsg:
		m.printing = false
		return m.emit()

	case quitMsg:
		m.quitting = true
		return m, tea.Quit

	case spinner.TickMsg:
		if !m.core.Busy() {
			m.spinning = false
			return m, nil
		}
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

func (m *model) remember(text string) {
	m.draft = ""
	if trimmed := strings.TrimSpace(text); trimmed != "" {
		if len(m.history) == 0 || m.history[len(m.history)-1] != text {
			m.history = append(m.history, text)
		}
	}
	m.histIdx = len(m.history)
}

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

func (m model) emit() (tea.Model, tea.Cmd) {
	if m.printing {
		return m, nil
	}

	if m.core.TranscriptLen() < m.printed {
		m.printed = 0
	}

	blocks := m.pending()
	if len(blocks) == 0 {
		return m, nil
	}
	m.printed += len(blocks)
	m.printing = true

	limit := m.printLimit()
	cmds := make([]tea.Cmd, 0, len(blocks)+1)
	for _, b := range blocks {
		for _, chunk := range chunkBlock(b, limit) {
			cmds = append(cmds, tea.Println(chunk))
		}
	}
	cmds = append(cmds, func() tea.Msg { return printedMsg{} })
	return m, tea.Sequence(cmds...)
}

func (m model) printLimit() int {
	if m.height == 0 {
		return 0
	}

	return max(m.height-lipgloss.Height(m.liveRegion()), 1)
}

func chunkBlock(block string, limit int) []string {
	lines := strings.Split(block, "\n")
	if limit <= 0 || len(lines) <= limit {
		return []string{block}
	}

	chunks := make([]string, 0, (len(lines)+limit-1)/limit)
	for len(lines) > limit {
		chunks = append(chunks, strings.Join(lines[:limit], "\n"))
		lines = lines[limit:]
	}

	return append(chunks, strings.Join(lines, "\n"))
}

func (m model) pending() []string {
	lines := m.core.Since(m.printed)
	blocks := make([]string, 0, len(lines))
	for _, ln := range lines {
		blocks = append(blocks, "\n"+m.renderBlock(ln))
	}
	return blocks
}

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
		return s.turnSep.Render(m.hrule) + "\n" + s.telemetry.Render(ln.Text)
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

func (m *model) trackThinking() tea.Cmd {
	if !m.core.Busy() {
		m.thinkingSince = time.Time{}
		return nil
	}
	if m.thinkingSince.IsZero() {
		m.thinkingSince = time.Now()
	}
	if m.spinning {
		return nil
	}
	m.spinning = true
	return m.spinner.Tick
}

func (m model) thinkingLine() string {
	if m.thinkingSince.IsZero() {
		return ""
	}
	elapsed := time.Since(m.thinkingSince).Truncate(time.Second)

	label := "Thinking for " + elapsed.String()
	if pending := m.core.PendingTool(); pending != "" {
		label = pending + " · " + elapsed.String()
	}

	return m.spinner.View() + m.styles.footer.Render(" "+label)
}

func (m model) placeholder() string {
	if m.core.Queued() {
		return queuedPlaceholder
	}
	return idlePlaceholder
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

// Run renders core in the terminal and blocks until the user quits with Ctrl+C
// or /exit, or until ctx is cancelled.
//
// It installs itself as the core's observer and sets the core's context, so
// there is no need to call chat.Chat.SetObserver or chat.Chat.SetContext
// beforehand. It returns the Bubble Tea program's error, or nil on a clean
// exit.
func Run(ctx context.Context, core *chat.Chat) error {
	core.SetContext(ctx)
	p := tea.NewProgram(newModel(core), tea.WithContext(ctx))
	core.SetObserver(&observer{program: p})
	_, err := p.Run()
	return err
}
