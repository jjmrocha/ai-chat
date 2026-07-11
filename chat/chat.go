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

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/mcp"
)

var (
	_ tea.Model      = model{}
	_ agent.Feedback = (*bridge)(nil)
)

type Config struct {
	Name        string
	Description string
	Font        string
	ThemeName   string
	MCP         mcpController
}

type mcpController interface {
	GetMCPs() []mcp.Status
	Start(ctx context.Context, name string) error
	Stop(name string) error
}

type agentController interface {
	Process(ctx context.Context, input string) (*agent.Response, error)
	ResetSession() error
	ModelInfo(ctx context.Context) *agent.ModelInfo
	ChangeModel(model string) error
	ChangeEffort(e llm.Effort)
	AvailableModels() []string
	CompactContext(ctx context.Context)
}

type (
	processDoneMsg struct {
		resp *agent.Response
		err  error
	}
	toolCalledMsg    struct{ name string }
	compactedMsg     struct{}
	compactFailedMsg struct{}
	mcpDoneMsg       struct {
		action string
		name   string
		err    error
	}
)

type bridge struct {
	program *tea.Program
}

func (b *bridge) send(msg tea.Msg) {
	if b.program != nil {
		b.program.Send(msg)
	}
}

func (b *bridge) ToolCalled(name string)   { b.send(toolCalledMsg{name: name}) }
func (b *bridge) ContextCompacted()        { b.send(compactedMsg{}) }
func (b *bridge) ContextCompactionFailed() { b.send(compactFailedMsg{}) }
func (b *bridge) SessionReset()            {}
func (b *bridge) SessionStarted()          {}
func (b *bridge) SessionClosed()           {}

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
	lastMeta   agent.Metadata
	width      int
	busy       bool
	ready      bool

	headerNameStyle lipgloss.Style
	userStyle       lipgloss.Style
	footerStyle     lipgloss.Style
	errorStyle      lipgloss.Style
	infoStyle       lipgloss.Style
	activityStyle   lipgloss.Style
	ruleStyle       lipgloss.Style
	turnSepStyle    lipgloss.Style
	telemetryStyle  lipgloss.Style
}

func newModel(ag agentController, cfg Config) model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()
	ti.Placeholder = "type /help for commands"

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(0),
	)

	m := model{
		ctx:      context.Background(),
		cfg:      cfg,
		agent:    ag,
		bridge:   &bridge{},
		renderer: renderer,
		viewport: viewport.New(),
		input:    ti,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
	m.applyTheme(cfg.ThemeName)
	return m
}

func (m *model) applyTheme(name string) {
	t, ok := lookupTheme(name)
	if !ok {
		t, ok = lookupTheme("default")
	}
	if !ok {
		t = Theme{
			HeaderName: "#FF9F1C",
			User:       "#FF9F1C",
			Footer:     "#94A3B8",
			Error:      "#F87171",
			Info:       "#00B4D8",
			Activity:   "#94A3B8",
			Rule:       "#4A5568",
			TurnSep:    "#4A5568",
			Telemetry:  "#94A3B8",
		}
	}
	m.headerNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.HeaderName)).Bold(true)
	m.userStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.User)).Bold(true)
	m.footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Footer)).Italic(true)
	m.errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error))
	m.infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Info))
	m.activityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Activity)).Italic(true)
	m.ruleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Rule))
	m.turnSepStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.TurnSep))
	m.telemetryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Telemetry)).Italic(true)
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
			m = m.appendBlock(m.errorStyle.Width(m.width).Render("Error: " + msg.err.Error()))
		} else {
			m.lastMeta = msg.resp.Metadata
			m = m.appendReply(msg.resp)
		}
		m = m.refresh()
		return m, nil

	case toolCalledMsg:
		m = m.appendBlock(m.activityStyle.Render("● tool: " + msg.name))
		m = m.refresh()
		return m, nil

	case compactedMsg:
		m = m.appendBlock(m.activityStyle.Render("● context compacted"))
		m = m.refresh()
		return m, nil

	case compactFailedMsg:
		m = m.appendBlock(m.errorStyle.Width(m.width).Render("Context compaction failed; will retry after the next turn."))
		m = m.refresh()
		return m, nil

	case mcpDoneMsg:
		if msg.err != nil {
			m = m.appendBlock(m.errorStyle.Width(m.width).Render("Error: " + msg.err.Error()))
		} else {
			verb := "started"
			if msg.action == "off" {
				verb = "stopped"
			}
			m = m.appendBlock(m.infoStyle.Width(m.width).Render("MCP " + msg.name + " " + verb + "."))
		}
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
			m.topBar(),
			m.input.View(),
			m.rule(m.width),
			m.statusLine(),
			"",
		)
		content = lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), frame)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeNone
	return v
}

func (m model) topBar() string {
	if m.cfg.Name == "" {
		return m.rule(m.width)
	}
	label := " " + m.cfg.Name + " "
	bar := strings.Repeat("─", max(0, m.width-lipgloss.Width(label))/2)
	return m.headerNameStyle.Render(bar + label + bar)
}

func (m model) submit(text string) (model, tea.Cmd) {
	text = strings.TrimSpace(text)
	if text == "" || m.busy {
		return m, nil
	}

	if strings.HasPrefix(text, "/") {
		return m.runCommand(text)
	}

	m = m.appendBlock(m.userStyle.Width(m.width).Render("❯ " + text))
	m.busy = true
	return m, tea.Batch(m.processCmd(text), m.spinner.Tick)
}

func (m model) runCommand(input string) (model, tea.Cmd) {
	name, args, _ := strings.Cut(strings.TrimPrefix(input, "/"), " ")
	args = strings.TrimSpace(args)

	switch name {
	case "clear":
		if err := m.agent.ResetSession(); err != nil {
			return m.appendBlock(m.errorStyle.Width(m.width).Render("Error: " + err.Error())), nil
		}
		m.transcript = nil
		return m.appendBlock(m.infoStyle.Width(m.width).Render("Context cleared.")), nil

	case "model":
		if args == "" {
			return m.appendBlock(m.infoStyle.Width(m.width).Render("Usage: /model <name>")), nil
		}
		if err := m.agent.ChangeModel(args); err != nil {
			return m.appendBlock(m.errorStyle.Width(m.width).Render("Error: " + err.Error())), nil
		}
		return m.appendBlock(m.infoStyle.Width(m.width).Render("Switched to: " + args)), nil

	case "models":
		models := m.agent.AvailableModels()
		if len(models) == 0 {
			return m.appendBlock(m.infoStyle.Width(m.width).Render("No model list available.")), nil
		}
		return m.appendBlock(m.infoStyle.Width(m.width).Render("Models: " + strings.Join(models, ", "))), nil

	case "effort":
		if args == "" {
			return m.appendBlock(m.infoStyle.Width(m.width).Render("Usage: /effort off|low|medium|max")), nil
		}
		var e llm.Effort
		switch llm.Effort(args) {
		case llm.EffortOff, llm.EffortLow, llm.EffortMedium, llm.EffortMax:
			e = llm.Effort(args)
		default:
			return m.appendBlock(m.errorStyle.Width(m.width).Render("Effort must be: off, low, medium, max")), nil
		}
		m.agent.ChangeEffort(e)
		return m.appendBlock(m.infoStyle.Width(m.width).Render("Effort: " + string(e))), nil

	case "compact":
		return m, m.compactCmd()

	case "mcp":
		return m.runMCPCommand(args)

	case "theme":
		if args == "" {
			names := themeNames()
			return m.appendBlock(m.infoStyle.Width(m.width).Render("Themes: " + strings.Join(names, ", "))), nil
		}
		if _, ok := lookupTheme(args); !ok {
			names := themeNames()
			return m.appendBlock(m.errorStyle.Width(m.width).Render("Unknown theme. Available: " + strings.Join(names, ", "))), nil
		}
		m.applyTheme(args)
		m = m.refresh()
		return m.appendBlock(m.infoStyle.Width(m.width).Render("Theme switched to: " + args)), nil

	case "exit":
		return m, tea.Quit

	case "help":
		help := strings.Join([]string{
			"Commands:",
			"  /model <name>   Switch model",
			"  /models         List available models",
			"  /effort <level> Set reasoning effort (off, low, medium, max)",
			"  /compact        Force context compaction",
			"  /mcp [on|off] [name]  Show or toggle MCP servers",
			"  /theme [name]   Show or switch color theme",
			"  /clear          Reset conversation",
			"  /help           Show this message",
			"  /exit           Quit",
		}, "\n")
		return m.appendBlock(m.infoStyle.Width(m.width).Render(help)), nil

	default:
		return m.appendBlock(m.errorStyle.Width(m.width).Render("Error: unknown command /" + name)), nil
	}
}

func (m model) processCmd(text string) tea.Cmd {
	return func() tea.Msg {
		resp, err := m.agent.Process(m.ctx, text)
		return processDoneMsg{resp: resp, err: err}
	}
}

// compactCmd runs compaction off the UI goroutine so the summarizing LLM call
// cannot block the event loop. The outcome is reported by the agent's feedback
// callbacks (compactedMsg / compactFailedMsg), so no message is emitted here.
func (m model) compactCmd() tea.Cmd {
	return func() tea.Msg {
		m.agent.CompactContext(m.ctx)
		return nil
	}
}

func (m model) runMCPCommand(args string) (model, tea.Cmd) {
	if m.cfg.MCP == nil {
		return m.appendBlock(m.infoStyle.Width(m.width).Render("No MCP servers configured.")), nil
	}

	action, name, _ := strings.Cut(args, " ")
	action = strings.TrimSpace(action)
	name = strings.TrimSpace(name)

	switch action {
	case "":
		statuses := m.cfg.MCP.GetMCPs()
		if len(statuses) == 0 {
			return m.appendBlock(m.infoStyle.Width(m.width).Render("No MCP servers registered.")), nil
		}
		lines := make([]string, 0, len(statuses)+1)
		lines = append(lines, "MCP servers:")
		for _, s := range statuses {
			state := "off"
			if s.Active {
				state = "on"
			}
			lines = append(lines, fmt.Sprintf("  %s: %s", s.Name, state))
		}
		return m.appendBlock(m.infoStyle.Width(m.width).Render(strings.Join(lines, "\n"))), nil

	case "on", "off":
		target, ok := m.resolveMCPName(name)
		if !ok {
			return m.appendBlock(m.errorStyle.Width(m.width).Render("Specify an MCP name: /mcp " + action + " <name>")), nil
		}
		return m, m.mcpCmd(action, target)

	default:
		return m.appendBlock(m.errorStyle.Width(m.width).Render("Usage: /mcp [on|off] [name]")), nil
	}
}

// resolveMCPName returns the given name, or the sole registered server's name
// when name is empty and exactly one is registered.
func (m model) resolveMCPName(name string) (string, bool) {
	if name != "" {
		return name, true
	}
	if statuses := m.cfg.MCP.GetMCPs(); len(statuses) == 1 {
		return statuses[0].Name, true
	}
	return "", false
}

// mcpCmd starts or stops an MCP off the UI goroutine; launching a server spawns
// a subprocess and performs a handshake, which must not block the event loop.
func (m model) mcpCmd(action, name string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch action {
		case "on":
			err = m.cfg.MCP.Start(m.ctx, name)
		case "off":
			err = m.cfg.MCP.Stop(name)
		}
		return mcpDoneMsg{action: action, name: name, err: err}
	}
}

func (m model) appendReply(resp *agent.Response) model {
	parts := []string{m.renderMarkdown(resp.Content)}
	if resp.Metadata.OutputTokens > 0 {
		sep := m.turnSepStyle.Render(strings.Repeat("━", m.width))
		parts = append(parts, sep, m.telemetryLine(resp.Metadata))
	}
	return m.appendBlock(strings.Join(parts, "\n"))
}

func (m model) telemetryLine(meta agent.Metadata) string {
	var parts []string
	if meta.ToolCalls > 0 {
		parts = append(parts, fmt.Sprintf("%d tool calls", meta.ToolCalls))
	}
	if meta.LLMDuration > 0 {
		parts = append(parts, fmt.Sprintf("%.1fs llm", meta.LLMDuration.Seconds()))
	}
	if meta.ToolDuration > 0 {
		parts = append(parts, fmt.Sprintf("%.1fs tools", meta.ToolDuration.Seconds()))
	}
	if meta.OutputTokens > 0 {
		parts = append(parts, fmt.Sprintf("%d out tok", meta.OutputTokens))
	}
	if len(parts) == 0 {
		return ""
	}
	return m.telemetryStyle.Render("[" + strings.Join(parts, " · ") + "]")
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
		return m.footerStyle.Render(m.spinner.View() + " thinking…")
	}

	name := "—"
	contextSize := 0
	var provider llm.Provider
	var effort llm.Effort
	if info := m.agent.ModelInfo(m.ctx); info != nil {
		if info.ModelName != "" {
			name = info.ModelName
		}
		provider = info.Provider
		contextSize = info.ModelContextSize
		effort = info.Effort
	}

	pct := 0.0
	if contextSize > 0 {
		pct = float64(m.lastMeta.TotalTokens) * 100 / float64(contextSize)
	}

	if provider != "" {
		name = fmt.Sprintf("%s (%s)", name, provider)
	}

	parts := []string{name}
	if effort != llm.EffortOff {
		parts = append(parts, string(effort))
	}
	parts = append(parts, fmt.Sprintf("ctx:%.0f%%", pct))
	parts = append(parts, fmt.Sprintf("%s tok", formatTokens(m.lastMeta.TotalTokens)))

	return m.footerStyle.Render(strings.Join(parts, " · "))
}

func (m model) rule(width int) string {
	if width <= 0 {
		return ""
	}
	return m.ruleStyle.Render(strings.Repeat("─", width))
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
	m.viewport.SetContent(strings.Join(m.transcript, "\n\n"))
	m.viewport.GotoBottom()
	return m
}
