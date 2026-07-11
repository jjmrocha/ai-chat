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
	"github.com/jjmrocha/ai-toolkit/agent"
)

var (
	_ agent.Feedback  = (*Chat)(nil)
	_ command.Context = (*Chat)(nil)
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

// agentRunner is the slice of *agent.Agent the core drives. Kept as an
// interface so the core is testable without a live model.
type agentRunner interface {
	Process(ctx context.Context, input string) (*agent.Response, error)
}

// Chat owns the transcript and mediates between the agent and the UI. All state
// is guarded by mu; observer notifications fire outside the lock.
type Chat struct {
	name  string
	agent agentRunner

	mu         sync.Mutex
	transcript []Line
	observer   Observer
	busy       bool
	lastMeta   agent.Metadata
}

// New builds a Chat over ag and installs itself as the agent's feedback sink so
// tool-call and compaction events flow into the transcript.
func New(name string, ag *agent.Agent) *Chat {
	c := &Chat{name: name, agent: ag}
	ag.SetFeedback(c)
	return c
}

// Name is the display name given at construction.
func (c *Chat) Name() string { return c.name }

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
