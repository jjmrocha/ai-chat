// Package chat is the headless core of the terminal chat. It owns the
// conversation transcript and drives an ai-toolkit agent, notifying a single
// Observer whenever the transcript changes so a UI can re-render. It has no
// dependency on any UI toolkit.
package chat

import (
	"context"
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

// Observer is notified after any transcript mutation. The UI implements it and
// re-renders; the core never renders itself.
type Observer interface {
	TranscriptChanged()
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

	mu         sync.Mutex
	transcript []Line
	observer   Observer
	busy       bool
	lastMeta   agent.Metadata
	theme      theme.Theme
}

// Option configures a Chat at construction.
type Option func(*Chat)

// WithTheme sets the color palette the UI applies. Defaults to theme.Default.
func WithTheme(t theme.Theme) Option {
	return func(c *Chat) { c.theme = t }
}

// newChat builds a Chat with defaults applied, then the options. It does not
// wire an agent, so tests can construct a core without a live model.
func newChat(name string, opts ...Option) *Chat {
	c := &Chat{name: name, ctx: context.Background(), theme: theme.Default}
	for _, opt := range opts {
		opt(c)
	}
	return c
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

// Submit runs text as a user turn unless it is blank or a turn is already in
// flight. The agent call runs off the caller's goroutine; results reach the
// transcript through the observer.
func (c *Chat) Submit(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	c.mu.Lock()
	if c.busy {
		c.mu.Unlock()
		return
	}
	c.busy = true
	c.mu.Unlock()
	go c.process(context.Background(), text)
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
	default:
		c.notify()
	}
}

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
