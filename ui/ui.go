// Package ui renders a chat.Chat as an inline Bubble Tea terminal program. It
// observes the core and prints each new transcript line above a live region
// holding the input and the status bar. Finished lines belong to the terminal's
// scrollback from then on, so selection, copying and wheel scrolling stay
// native; the program never enters the alternate screen or captures the mouse.
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
	"github.com/jjmrocha/ai-chat/theme"
)

const (
	// maxInputLines caps how far the input grows before it starts scrolling.
	maxInputLines = 6
	// inputIndent pads every row of the input, standing in for the prompt marker
	// the input no longer carries and lining it up with the indented replies.
	inputIndent = "  "
	// idlePlaceholder invites the first message; queuedPlaceholder replaces it
	// once input is parked behind the turn in flight, which is the only sign the
	// user gets that what they typed was taken but has not started yet.
	idlePlaceholder   = "Send a message…  (/help for commands)"
	queuedPlaceholder = "(queued)"
)

type (
	refreshMsg struct{}
	quitMsg    struct{}
	// printedMsg closes a print sequence. Bubble Tea runs the commands an Update
	// returns on their own goroutines, so two print sequences in flight can reach
	// the terminal in either order; this one reports that the sequence landed and
	// the next may start.
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
		turnSep:    fg(t.TurnSep),
		footer:     fg(t.Footer).Italic(true),
	}
}

type chatCore interface {
	Name() string
	Transcript() []chat.Line
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

	// printed counts the transcript lines already sent to the scrollback. The
	// transcript is append-only apart from /clear, so this doubles as the mark
	// separating printed history from what still has to go out.
	printed int

	// thinkingSince marks when the current wait on the agent began, or is zero
	// while the agent is idle. The spinner already ticks several times a second,
	// so the elapsed count re-renders without a clock of its own.
	thinkingSince time.Time

	// printing is set while a print sequence is on its way to the terminal. Only
	// one may be in flight, so blocks appended meanwhile wait for the printedMsg
	// that closes it rather than racing ahead of it.
	printing bool

	// quitting blanks the live region for the final render, so the shell prompt
	// comes back under the conversation instead of under a stale input box.
	quitting bool

	// history holds submitted prompts, oldest first. histIdx points at the one
	// currently recalled; when it equals len(history) the user is editing their
	// own text rather than browsing, and draft is empty.
	history []string
	histIdx int
	draft   string
}

func newModel(core chatCore) model {
	sty := newStyles(theme.Palette)

	ti := textarea.New()
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
	ti.SetStyles(inputStyles(theme.Palette))
	ti.Focus()

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Palette.Info))

	return model{
		core:     core,
		styles:   sty,
		input:    ti,
		spinner:  sp,
		renderer: newRenderer(0),
	}
}

// inputStyles strips the textarea's painted background and fixed foregrounds.
// textarea.New installs DefaultDarkStyles, whose focused CursorLine paints an
// opaque ANSI-0 block behind the line being typed and whose Prompt is ANSI 7;
// on a light profile that is dark text on black under a white-on-white prompt.
// Letting the terminal's own background and text colour through is correct on
// any profile. The cursor takes the palette's accent so it stays findable, and
// the placeholder takes TurnSep, the dimmest value in the palette, so prompt
// text never reads as something the user typed.
func inputStyles(t theme.Theme) textarea.Styles {
	s := textarea.DefaultDarkStyles()
	for _, state := range []*textarea.StyleState{&s.Focused, &s.Blurred} {
		state.Base = lipgloss.NewStyle()
		state.CursorLine = lipgloss.NewStyle()
		state.Prompt = lipgloss.NewStyle()
		state.Text = lipgloss.NewStyle()
		state.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color(t.TurnSep))
	}
	s.Cursor.Color = lipgloss.Color(t.Info)
	return s
}

// newRenderer builds the markdown renderer. The style is fixed: glamour's other
// builtins are hex palettes tuned for one background, and this code has no way
// to know the user's, while notty sets no colour at all. The cost is that it
// conveys emphasis by re-emitting the markup — "**bold**" keeps its asterisks —
// and highlights no syntax.
func newRenderer(width int) *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("notty"),
		glamour.WithWordWrap(width),
	)
	return r
}

func (m model) Init() tea.Cmd { return tea.Batch(textarea.Blink, m.spinner.Tick) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.trackThinking()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width != m.width {
			m.renderer = newRenderer(msg.Width)
		}
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(max(msg.Width-2, 0))
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
		m.input.Placeholder = m.placeholder()
		content = m.liveRegion()
	}

	v := tea.NewView(content)
	v.AltScreen = false
	v.MouseMode = tea.MouseModeNone
	return v
}

// liveRegion is everything held below the scrollback: the thinking row, the
// title bar, the input, and the status bar. The thinking row opens it and keeps
// its line while the agent idles, so the layout never jumps and printed blocks
// always have a blank line between them and the title bar.
func (m model) liveRegion() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		m.thinkingLine(),
		m.titleBar(),
		m.input.View(),
		m.styles.headerName.Render(m.hrule()),
		m.styles.footer.Render(m.core.StatusText()),
	)
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
	if m.printing {
		return m, nil
	}

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

// printLimit is how many lines one printed block may carry. Bubble Tea's inline
// renderer makes room for a block by scrolling the live region out of the way in
// a single insert, and a terminal clamps that scroll at the top of the screen:
// ask for more rows than are free and the live region is left stranded in the
// scrollback, trailed by the blank lines the insert could not fill. Zero means
// the height is not known yet, so nothing is split.
func (m model) printLimit() int {
	if m.height == 0 {
		return 0
	}

	return max(m.height-lipgloss.Height(m.liveRegion()), 1)
}

// chunkBlock splits a block into pieces no taller than limit lines, each of
// which the renderer can make room for on its own. A limit of zero or less
// leaves the block whole. Rejoining the pieces with a newline reproduces the
// block, so nothing is added or lost by the split.
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

// pending renders the transcript lines that have not reached the terminal yet.
func (m model) pending() []string {
	lines := m.core.Transcript()
	if len(lines) <= m.printed {
		return nil
	}
	blocks := make([]string, 0, len(lines)-m.printed)
	for _, ln := range lines[m.printed:] {
		// A leading blank line gives every block room of its own. Closing one
		// too would double the gap, since the next block opens with its own.
		blocks = append(blocks, "\n"+m.renderBlock(ln))
	}
	return blocks
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
		return m.renderActivity(ln.Text)
	case command.Telemetry:
		return s.turnSep.Render(m.hrule()) + "\n" + s.telemetry.Render(ln.Text)
	case command.Reply:
		return m.renderMarkdown(ln.Text)
	default:
		return ln.Text
	}
}

// renderActivity styles a tool call and the response line that closes it apart,
// so the results read dimmer than the calls that produced them. An activity
// carrying no response — a compaction notice — is styled whole.
func (m model) renderActivity(text string) string {
	call, response, found := strings.Cut(text, "\n")
	if !found {
		return m.styles.activity.Render(text)
	}

	return m.styles.activity.Render(call) + "\n" + m.styles.telemetry.Render(response)
}

// trackThinking starts the wait clock when a turn begins and stops it when the
// agent goes idle. A queued drain is one wait: the clock spans it rather than
// restarting per turn, since what it reports is how long the user has waited.
func (m *model) trackThinking() {
	switch {
	case !m.core.Busy():
		m.thinkingSince = time.Time{}
	case m.thinkingSince.IsZero():
		m.thinkingSince = time.Now()
	}
}

// thinkingLine is the row above the title bar: the spinner and how long the wait
// has run, or an empty line holding that row while the agent idles.
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

// placeholder is the prompt an empty input shows.
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

// Run renders core in a Bubble Tea program until the user quits or ctx is done.
func Run(ctx context.Context, core *chat.Chat) error {
	core.SetContext(ctx)
	p := tea.NewProgram(newModel(core), tea.WithContext(ctx))
	core.SetObserver(&observer{program: p})
	_, err := p.Run()
	return err
}
