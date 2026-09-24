package chat

import (
	"context"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
)

var (
	_ command.Context         = (*Chat)(nil)
	_ command.AgentController = (*Chat)(nil)
)

// Print appends a line to the transcript and notifies the observer. It
// implements [command.Context] so commands can write output; text should carry
// no decoration, since the front-end styles it by kind.
func (c *Chat) Print(kind command.Kind, text string) { c.append(kind, text) }

// Agent returns the controller commands use to reach the agent. It implements
// [command.Context].
func (c *Chat) Agent() command.AgentController { return c }

// Clear resets the agent session and empties the transcript, discarding the
// last turn's metadata. It implements [command.Context] for /clear and returns
// the agent's error without clearing anything if the reset fails.
func (c *Chat) Clear() error {
	if err := c.agent.ResetSession(); err != nil {
		return err
	}
	c.mutate(func() {
		c.transcript = nil
		c.epoch++
		c.lastMeta = agent.Metadata{}
	})
	return nil
}

// ChangeModel switches the agent to the named model, which must be one of
// [Chat.AvailableModels]. It implements [command.AgentController] for /model
// and returns the agent's error unchanged on failure.
func (c *Chat) ChangeModel(name string) error {
	err := c.agent.ChangeModel(name)
	c.invalidateModel()
	return err
}

// ChangeEffort switches the agent's reasoning effort. It implements
// [command.AgentController] for /effort and returns the agent's error for an
// unsupported level.
func (c *Chat) ChangeEffort(e llm.Effort) error {
	err := c.agent.ChangeEffort(e)
	c.invalidateModel()
	return err
}

// AvailableModels returns the models the agent can switch to. It implements
// [command.AgentController] for /model.
func (c *Chat) AvailableModels() []string { return c.agent.AvailableModels() }

// Compact forces context compaction under the running command's context, so
// [Chat.Cancel] interrupts it. It implements [command.AgentController] for
// /compact; the agent reports the outcome through the feedback methods.
func (c *Chat) Compact() { c.agent.CompactContext(c.Context()) }

// Context returns the context of the running turn or command, which
// [Chat.Cancel] and [Chat.Close] cancel, or the session's context when nothing
// is running. It implements [command.Context].
func (c *Chat) Context() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.workCtx != nil {
		return c.workCtx
	}
	return c.baseCtx
}
