package chat

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	figure "github.com/common-nighthawk/go-figure"

	"github.com/jjmrocha/ai-toolkit/agent"
)

const defaultFont = "small"

var (
	_ tea.Model      = model{}
	_ agent.Feedback = (*bridge)(nil)
)

type Config struct {
	Name        string
	Description string
	// Font is the FIGlet font used to render Name as a block-letter banner.
	// Empty selects "banner3".
	Font string
}

type agentController interface {
	Process(ctx context.Context, input string) (*agent.Response, error)
	ResetSession() error
}

type (
	processDoneMsg struct {
		resp *agent.Response
		err  error
	}
	toolCalledMsg struct{ name string }
	compactedMsg  struct{}
)

type bridge struct {
	program *tea.Program
}

func (b *bridge) send(msg tea.Msg) {
	if b.program != nil {
		b.program.Send(msg)
	}
}

func (b *bridge) ToolCalled(name string) { b.send(toolCalledMsg{name: name}) }
func (b *bridge) ContextCompacted()      { b.send(compactedMsg{}) }
func (b *bridge) SessionReset()          {}
func (b *bridge) SessionStarted()        {}
func (b *bridge) SessionClosed()         {}

var (
	headerNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	headerDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	userStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	footerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	infoStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	activityStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	ruleStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

const frameHeight = 5 // top rule + input + bottom rule + status + blank

type model struct {
	ctx        context.Context
	cfg        Config
	agent      agentController
	bridge     *bridge
	renderer   *glamour.TermRenderer
	viewport   viewport.Model
	input      textinput.Model
	spinner    spinner.Model
	transcript []string
	banner     string
	lastMeta   agent.Metadata
	width      int
	busy       bool
	ready      bool
}

func newModel(ag agentController, cfg Config) model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(0),
	)

	var banner string
	if cfg.Name != "" {
		font := cfg.Font
		if font == "" {
			font = defaultFont
		}
		banner = strings.Trim(figure.NewFigure(cfg.Name, font, true).String(), "\n")
	}

	return model{
		ctx:      context.Background(),
		cfg:      cfg,
		agent:    ag,
		bridge:   &bridge{},
		renderer: renderer,
		viewport: viewport.New(),
		input:    ti,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
		banner:   banner,
	}
}

type Model struct {
	m model
}

func New(a *agent.Agent, cfg Config) *Model {
	m := newModel(a, cfg)
	a.SetFeedback(m.bridge)
	return &Model{m: m}
}

func (ui *Model) Run(ctx context.Context) error {
	ui.m.ctx = ctx
	p := tea.NewProgram(ui.m, tea.WithContext(ctx))
	ui.m.bridge.program = p
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m = m.resize(msg.Width, msg.Height)
		m.ready = true
		m = m.refresh()
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			text := m.input.Value()
			m.input.Reset()
			var cmd tea.Cmd
			m, cmd = m.submit(text)
			m = m.refresh()
			return m, cmd
		}

	case processDoneMsg:
		m.busy = false
		if msg.err != nil {
			m = m.appendBlock(errorStyle.Width(m.width).Render("Error: " + msg.err.Error()))
		} else {
			m.lastMeta = msg.resp.Metadata
			m = m.appendReply(msg.resp)
		}
		m = m.refresh()
		return m, nil

	case toolCalledMsg:
		m = m.appendBlock(activityStyle.Render("● tool: " + msg.name))
		m = m.refresh()
		return m, nil

	case compactedMsg:
		m = m.appendBlock(activityStyle.Render("● context compacted"))
		m = m.refresh()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
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
		frame := lipgloss.JoinVertical(lipgloss.Left,
			rule(m.width),
			m.input.View(),
			rule(m.width),
			m.statusLine(),
			"",
		)
		content = lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), frame)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	// Mouse capture stays off so the terminal keeps native text selection and
	// copy; the viewport scrolls from the keyboard (and the terminal's
	// alternate-scroll wheel, which arrives as arrow keys).
	v.MouseMode = tea.MouseModeNone
	return v
}

func (m model) submit(text string) (model, tea.Cmd) {
	text = strings.TrimSpace(text)
	if text == "" || m.busy {
		return m, nil
	}

	if strings.HasPrefix(text, "/") {
		return m.runCommand(text)
	}

	m = m.appendBlock(userStyle.Width(m.width).Render("❯ " + text))
	m.busy = true
	return m, tea.Batch(m.processCmd(text), m.spinner.Tick)
}

func (m model) runCommand(input string) (model, tea.Cmd) {
	name, _, _ := strings.Cut(strings.TrimPrefix(input, "/"), " ")

	switch name {
	case "clear":
		if err := m.agent.ResetSession(); err != nil {
			return m.appendBlock(errorStyle.Width(m.width).Render("Error: " + err.Error())), nil
		}
		m.transcript = nil
		return m.appendBlock(infoStyle.Width(m.width).Render("Context cleared.")), nil
	case "exit":
		return m, tea.Quit
	default:
		return m.appendBlock(errorStyle.Width(m.width).Render("Error: unknown command /" + name)), nil
	}
}

func (m model) processCmd(text string) tea.Cmd {
	return func() tea.Msg {
		resp, err := m.agent.Process(m.ctx, text)
		return processDoneMsg{resp: resp, err: err}
	}
}

func (m model) appendReply(resp *agent.Response) model {
	return m.appendBlock(m.renderMarkdown(resp.Content))
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

func (m model) statusLine() string {
	if m.busy {
		return footerStyle.Render(m.spinner.View() + " thinking…")
	}

	name := m.lastMeta.ModelName
	if name == "" {
		name = "—"
	}

	pct := 0.0
	if m.lastMeta.ModelContextSize > 0 {
		pct = float64(m.lastMeta.TotalTokens) * 100 / float64(m.lastMeta.ModelContextSize)
	}

	status := fmt.Sprintf("%s | Context: %.1f%% | Tokens: %s", name, pct, formatTokens(m.lastMeta.TotalTokens))
	return footerStyle.Render(status)
}

func (m model) header() string {
	if m.cfg.Name == "" && m.cfg.Description == "" {
		return ""
	}

	lines := make([]string, 0, 3)
	if m.banner != "" {
		lines = append(lines, headerNameStyle.Render(centerBlock(m.banner, m.width)))
	}
	if m.cfg.Description != "" {
		lines = append(lines, headerDescStyle.Width(m.width).Align(lipgloss.Center).Render(m.cfg.Description))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// centerBlock left-pads every line of s by the same amount so the block is
// centered within width as a unit, preserving the internal alignment of
// multi-line art (centering each line independently would shear it).
func centerBlock(s string, width int) string {
	lines := strings.Split(s, "\n")
	maxw := 0
	for _, ln := range lines {
		if w := lipgloss.Width(ln); w > maxw {
			maxw = w
		}
	}
	if maxw >= width {
		return s
	}
	pad := strings.Repeat(" ", (width-maxw)/2)
	for i, ln := range lines {
		lines[i] = pad + ln
	}
	return strings.Join(lines, "\n")
}

func rule(width int) string {
	if width <= 0 {
		return ""
	}
	return ruleStyle.Render(strings.Repeat("─", width))
}

func (m model) appendBlock(s string) model {
	m.transcript = append(m.transcript, s)
	return m
}

func (m model) resize(width, height int) model {
	m.width = width
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(max(height-frameHeight, 0))
	m.input.SetWidth(max(width-2, 0))
	if r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	); err == nil {
		m.renderer = r
	}
	return m
}

func (m model) refresh() model {
	blocks := m.transcript
	if h := m.header(); h != "" {
		blocks = append([]string{h, ""}, m.transcript...)
	}
	m.viewport.SetContent(strings.Join(blocks, "\n\n"))
	m.viewport.GotoBottom()
	return m
}
