package ui

import (
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"

	"github.com/jjmrocha/ai-chat/chat"
)

const (
	maxInputLines = 6

	inputIndent = "  "
)

type model struct {
	core     chatCore
	styles   styles
	input    textarea.Model
	spinner  spinner.Model
	renderer *glamour.TermRenderer
	width    int
	height   int
	ready    bool

	cursor        chat.Cursor
	thinkingSince time.Time
	printing      bool
	quitting      bool
	spinning      bool

	hrule     string
	printRule string
	titleBar  string

	history history
}

func newModel(core chatCore) model {
	return model{
		core:     core,
		styles:   newStyles(),
		input:    newInput(),
		spinner:  newSpinner(),
		renderer: newRenderer(0),
	}
}

func newInput() textarea.Model {
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
	return ti
}

func newSpinner() spinner.Model {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = fg(defaultPalette.Info)
	return sp
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
		m.setSize(msg.Width, msg.Height)
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
		return m, m.spin(msg)

	case tea.KeyPressMsg:
		if cmd, handled := m.handleKey(msg); handled {
			return m, cmd
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) setSize(width, height int) {
	resized := width != m.width
	m.width = width
	m.height = height
	if resized {
		m.renderer = newRenderer(m.printWidth())
	}
	m.input.SetWidth(max(width-2, 0))
	m.resize()
	m.ready = true
}

func (m *model) spin(tick spinner.TickMsg) tea.Cmd {
	if !m.core.Busy() {
		m.spinning = false
		return nil
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(tick)
	return cmd
}

func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return tea.Quit, true
	case "esc":
		if m.core.Busy() {
			m.core.Cancel()
		}
		return nil, true
	case "enter":
		text := m.input.Value()
		m.input.Reset()
		m.remember(text)
		m.core.Submit(text)
		return nil, true
	case "up":
		return nil, m.recallOlder()
	case "down":
		return nil, m.recallNewer()
	}
	return nil, false
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
