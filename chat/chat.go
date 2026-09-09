// Package chat is the headless core of the terminal chat. It owns the
// conversation transcript and drives an ai-toolkit agent, notifying a single
// Observer whenever the transcript changes so a UI can re-render. It has no
// dependency on any UI toolkit.
//
// The Chat type is split across files by concern: this file holds the core
// state and input lifecycle, context.go the command.Context capabilities,
// feedback.go the agent.Feedback events, and format.go the status and
// telemetry formatting.
package chat

import (
	"context"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-chat/theme"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
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

type agentBackend interface {
	Process(ctx context.Context, input string) (*agent.Response, error)
	ChangeModel(name string) error
	ChangeEffort(e llm.Effort) error
	AvailableModels() []string
	ModelInfo(ctx context.Context) *agent.ModelInfo
	CompactContext(ctx context.Context)
	ResetSession() error
}

// Chat owns the transcript and mediates between the agent and the UI. All state
// is guarded by mu; observer notifications fire outside the lock.
type Chat struct {
	name    string
	agent   agentBackend
	baseCtx context.Context

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

// New builds a Chat over ag and installs itself as the agent's feedback sink so
// tool-call and compaction events flow into the transcript.
func New(name string, ag *agent.Agent, opts ...Option) *Chat {
	c := newChat(name, opts...)
	c.agent = ag
	ag.SetFeedback(c)
	return c
}

func newChat(name string, opts ...Option) *Chat {
	c := &Chat{
		name:         name,
		baseCtx:      context.Background(),
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

// SetContext replaces the context used for agent calls. Must be called before
// the first Submit. Not safe for concurrent use.
func (c *Chat) SetContext(ctx context.Context) { c.baseCtx = ctx }

func (c *Chat) register(cmd command.Command) {
	c.commands[cmd.Name()] = cmd
}

// Name is the display name given at construction.
func (c *Chat) Name() string { return c.name }

// Theme returns the active color palette.
func (c *Chat) Theme() theme.Theme {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.theme
}

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
	go c.process(c.baseCtx, text)
}

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

func (c *Chat) helpText() string {
	names := make([]string, 0, len(c.commands))
	for name := range c.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	type entry struct{ usage, desc string }
	entries := make([]entry, 0, len(names)+2)
	for _, name := range names {
		cmd := c.commands[name]
		entries = append(entries, entry{usage: usageOf(cmd), desc: cmd.Help()})
	}
	entries = append(entries,
		entry{usage: "/help", desc: "Show this message"},
		entry{usage: "/exit", desc: "Quit"},
	)

	width := 0
	for _, e := range entries {
		width = max(width, utf8.RuneCountInString(e.usage))
	}

	lines := make([]string, 0, len(entries)+1)
	lines = append(lines, "Commands:")
	for _, e := range entries {
		pad := strings.Repeat(" ", width-utf8.RuneCountInString(e.usage))
		lines = append(lines, "  "+e.usage+pad+" "+e.desc)
	}
	return strings.Join(lines, "\n")
}

// usageOf renders a command as it appears in the left column of /help: its name
// plus the argument spec, when it advertises one.
func usageOf(cmd command.Command) string {
	usage := "/" + cmd.Name()
	if a, ok := cmd.(command.Argumented); ok && a.Args() != "" {
		usage += " " + a.Args()
	}
	return usage
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
		c.append(command.Error, "No response received.")
	}
}

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
