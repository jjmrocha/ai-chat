// Package chat is the headless core of the terminal chat. It owns the
// conversation transcript and drives an ai-toolkit agent, notifying a single
// Observer whenever the transcript changes so a UI can re-render. It has no
// dependency on any UI toolkit.
package chat

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-chat/theme"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
)

var (
	_ agent.Feedback          = (*Chat)(nil)
	_ command.Context         = (*Chat)(nil)
	_ command.AgentController = (*Chat)(nil)
)

// Line is one transcript entry: its text and the Kind the UI styles it by.
type Line struct {
	Kind command.Kind
	Text string
}

// Observer receives the core's two signals to the UI: re-render after a
// transcript change, and quit. The UI implements both; the core never renders
// or exits the program itself.
type Observer interface {
	TranscriptChanged()
	Quit()
}

// agentBackend is the slice of *agent.Agent the core drives. Kept as an
// interface so the core is testable without a live model.
type agentBackend interface {
	Process(ctx context.Context, input string) (*agent.Response, error)
	ChangeModel(name string) error
	ChangeEffort(e llm.Effort)
	AvailableModels() []string
	ModelInfo(ctx context.Context) *agent.ModelInfo
	CompactContext(ctx context.Context)
	ResetSession() error
}

// Chat owns the transcript and mediates between the agent and the UI. All state
// is guarded by mu; observer notifications fire outside the lock.
type Chat struct {
	name  string
	agent agentBackend
	ctx   context.Context

	commands     map[string]command.Command
	telemetryFmt TelemetryFormatter
	statusFmt    StatusFormatter

	mu         sync.Mutex
	transcript []Line
	observer   Observer
	busy       bool
	lastMeta   agent.Metadata
	theme      theme.Theme
}

// newChat builds a Chat with defaults applied, then the options. It does not
// wire an agent, so tests can construct a core without a live model.
func newChat(name string, opts ...Option) *Chat {
	c := &Chat{
		name:         name,
		ctx:          context.Background(),
		theme:        theme.Default,
		commands:     map[string]command.Command{},
		telemetryFmt: defaultTelemetryFormatter,
		statusFmt:    defaultStatusFormatter,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// register adds cmd to the command table, keyed by its name.
func (c *Chat) register(cmd command.Command) {
	c.commands[cmd.Name()] = cmd
}

// New builds a Chat over ag and installs itself as the agent's feedback sink so
// tool-call and compaction events flow into the transcript.
func New(name string, ag *agent.Agent, opts ...Option) *Chat {
	c := newChat(name, opts...)
	c.agent = ag
	ag.SetFeedback(c)
	return c
}

// Name is the display name given at construction.
func (c *Chat) Name() string { return c.name }

// Theme returns the color palette selected at construction.
func (c *Chat) Theme() theme.Theme { return c.theme }

// SetObserver registers the single observer notified on transcript changes.
func (c *Chat) SetObserver(o Observer) {
	c.mu.Lock()
	c.observer = o
	c.mu.Unlock()
}

// Transcript returns a snapshot copy of the current transcript.
func (c *Chat) Transcript() []Line {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Line, len(c.transcript))
	copy(out, c.transcript)
	return out
}

// Busy reports whether an agent turn is currently in flight.
func (c *Chat) Busy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.busy
}

// LastMetadata returns the metadata of the most recently completed turn.
func (c *Chat) LastMetadata() agent.Metadata {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastMeta
}

// Submit handles a line of user input: a slash command is dispatched, otherwise
// the text runs as an agent turn. Blank input and input arriving while a turn is
// already in flight are ignored. Agent and command work run off the caller's
// goroutine; results reach the transcript through the observer.
func (c *Chat) Submit(text string) {
	text = strings.TrimSpace(text)
	if text == "" || c.Busy() {
		return
	}
	if strings.HasPrefix(text, "/") {
		c.dispatch(text)
		return
	}
	c.mu.Lock()
	c.busy = true
	c.mu.Unlock()
	go c.process(c.ctx, text)
}

// dispatch parses and runs a slash command. The always-present /exit and /help
// are handled here; any other name is looked up in the registered commands and
// run on its own goroutine so a slow command never blocks rendering.
func (c *Chat) dispatch(input string) {
	name, args, _ := strings.Cut(strings.TrimPrefix(input, "/"), " ")
	args = strings.TrimSpace(args)

	switch name {
	case "exit":
		c.quit()
		return
	case "help":
		c.append(command.Info, c.helpText())
		return
	}

	cmd, ok := c.commands[name]
	if !ok {
		c.append(command.Error, "Error: unknown command /"+name)
		return
	}
	go cmd.Run(c, args)
}

// helpText lists the registered commands (sorted) plus the always-present ones.
func (c *Chat) helpText() string {
	names := make([]string, 0, len(c.commands))
	for name := range c.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	lines := make([]string, 0, len(names)+3)
	lines = append(lines, "Commands:")
	for _, name := range names {
		lines = append(lines, "  "+c.commands[name].Help())
	}
	lines = append(lines,
		"  /help           Show this message",
		"  /exit           Quit",
	)
	return strings.Join(lines, "\n")
}

func (c *Chat) quit() {
	c.mu.Lock()
	o := c.observer
	c.mu.Unlock()
	if o != nil {
		o.Quit()
	}
}

func (c *Chat) process(ctx context.Context, text string) {
	c.append(command.User, "❯ "+text)

	resp, err := c.agent.Process(ctx, text)

	c.mu.Lock()
	c.busy = false
	if err == nil && resp != nil {
		c.lastMeta = resp.Metadata
	}
	c.mu.Unlock()

	switch {
	case err != nil:
		c.append(command.Error, "Error: "+err.Error())
	case resp != nil:
		c.append(command.Reply, resp.Content)
		if line := c.telemetryFmt(resp.Metadata); line != "" {
			c.append(command.Telemetry, line)
		}
	default:
		c.notify()
	}
}

// Status assembles the current status data from the agent and last turn.
func (c *Chat) Status() StatusInfo {
	meta := c.LastMetadata()
	info := StatusInfo{Tokens: meta.TotalTokens}
	if mi := c.agent.ModelInfo(c.ctx); mi != nil {
		info.Name = mi.ModelName
		info.Provider = mi.Provider
		info.Effort = mi.Effort
		if mi.ModelContextSize > 0 {
			info.CtxPct = float64(meta.TotalTokens) * 100 / float64(mi.ModelContextSize)
		}
	}
	return info
}

// StatusText renders the status bar as plain text via the status formatter.
func (c *Chat) StatusText() string { return c.statusFmt(c.Status()) }

// Print implements command.Context.
func (c *Chat) Print(kind command.Kind, text string) { c.append(kind, text) }

// Agent implements command.Context, exposing the agent operations commands may
// drive. The core is its own controller, supplying the request context.
func (c *Chat) Agent() command.AgentController { return c }

// Clear implements command.Context: reset the agent session and empty the
// transcript. On reset failure the transcript is left intact.
func (c *Chat) Clear() error {
	if err := c.agent.ResetSession(); err != nil {
		return err
	}
	c.mu.Lock()
	c.transcript = nil
	c.mu.Unlock()
	c.notify()
	return nil
}

// ChangeTheme implements command.Context: switch the active color palette.
func (c *Chat) ChangeTheme(name string) error {
	t, ok := theme.ByName(name)
	if !ok {
		return fmt.Errorf("unknown theme %q", name)
	}
	c.mu.Lock()
	c.theme = t
	c.mu.Unlock()
	c.notify()
	return nil
}

// ChangeModel implements command.AgentController.
func (c *Chat) ChangeModel(name string) error { return c.agent.ChangeModel(name) }

// ChangeEffort implements command.AgentController.
func (c *Chat) ChangeEffort(e llm.Effort) { c.agent.ChangeEffort(e) }

// AvailableModels implements command.AgentController.
func (c *Chat) AvailableModels() []string { return c.agent.AvailableModels() }

// ModelInfo implements command.AgentController.
func (c *Chat) ModelInfo() *agent.ModelInfo { return c.agent.ModelInfo(c.ctx) }

// Compact implements command.AgentController: run context compaction. The
// outcome arrives through the agent's feedback events.
func (c *Chat) Compact() { c.agent.CompactContext(c.ctx) }

// ToolCalled implements agent.Feedback.
func (c *Chat) ToolCalled(name string) { c.append(command.Activity, "● tool: "+name) }

// ContextCompacted implements agent.Feedback.
func (c *Chat) ContextCompacted() { c.append(command.Activity, "● context compacted") }

// ContextCompactionFailed implements agent.Feedback.
func (c *Chat) ContextCompactionFailed() {
	c.append(command.Error, "Context compaction failed; will retry after the next turn.")
}

// SessionReset implements agent.Feedback.
func (c *Chat) SessionReset() {}

// SessionStarted implements agent.Feedback.
func (c *Chat) SessionStarted() {}

// SessionClosed implements agent.Feedback.
func (c *Chat) SessionClosed() {}

func (c *Chat) append(k command.Kind, text string) {
	c.mu.Lock()
	c.transcript = append(c.transcript, Line{Kind: k, Text: text})
	c.mu.Unlock()
	c.notify()
}

func (c *Chat) notify() {
	c.mu.Lock()
	o := c.observer
	c.mu.Unlock()
	if o != nil {
		o.TranscriptChanged()
	}
}
