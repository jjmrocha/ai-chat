package chat

import (
	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/agent"
)

var _ agent.Feedback = (*Chat)(nil)

// ToolCalled implements agent.Feedback.
func (c *Chat) ToolCalled(name string) { c.append(command.Activity, "● tool: "+name) }

// ContextCompacted implements agent.Feedback.
func (c *Chat) ContextCompacted() { c.append(command.Activity, "● context compacted") }

// ContextCompactionFailed implements agent.Feedback.
func (c *Chat) ContextCompactionFailed() {
	c.append(command.Error, "Context compaction failed; will retry after the next turn.")
}

// ModelInfoUnavailable implements agent.Feedback.
func (c *Chat) ModelInfoUnavailable() {
	c.append(command.Error, "Model info unavailable; automatic context compaction is disabled.")
}

// SessionReset implements agent.Feedback.
func (c *Chat) SessionReset() {}

// SessionStarted implements agent.Feedback.
func (c *Chat) SessionStarted() {}

// SessionClosed implements agent.Feedback.
func (c *Chat) SessionClosed() {}
