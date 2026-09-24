package chat

import (
	"time"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/agent"
)

var _ agent.Feedback = (*Chat)(nil)

// PendingTool returns a description of the tool call in flight, or an empty
// string when none is running. A front-end can show it as progress detail.
func (c *Chat) PendingTool() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pendingTool
}

// ToolCalled records that the agent started a tool call, making it visible
// through [Chat.PendingTool]. It implements agent.Feedback and is called by the
// agent, not by your code.
func (c *Chat) ToolCalled(name string, args map[string]any) {
	c.mutate(func() { c.pendingTool = sanitize(formatToolCall(name, args)) })
}

// ToolReturned records the outcome of the tool call in flight, appending it as
// one [command.Activity] line whose Text is the call and whose Detail is the
// result. It implements agent.Feedback and is called by the agent.
func (c *Chat) ToolReturned(_ string, result string, err error, elapsed time.Duration) {
	c.closeToolCall(formatToolResult(result, err, elapsed))
}

func (c *Chat) closeToolCall(response string) {
	c.mu.Lock()
	request := c.pendingTool
	c.pendingTool = ""
	c.mu.Unlock()

	if request == "" {
		return
	}

	c.appendLine(Line{Kind: command.Activity, Text: request, Detail: response})
}

// TokensUsed updates the token count shown in the status bar after each
// intermediate model response. It implements agent.Feedback and is called by
// the agent.
func (c *Chat) TokensUsed(totalTokens int) {
	c.mutate(func() { c.lastMeta.TotalTokens = totalTokens })
}

// ContextCompacted notes a successful context compaction in the transcript. It
// implements agent.Feedback and is called by the agent.
func (c *Chat) ContextCompacted() { c.append(command.Info, "Context compacted.") }

// ContextCompactionFailed notes a failed context compaction in the transcript.
// It implements agent.Feedback and is called by the agent.
func (c *Chat) ContextCompactionFailed() {
	c.append(command.Error, "Context compaction failed; will retry after the next turn.")
}

// ModelInfoUnavailable notes that the model's limits could not be read, which
// disables automatic compaction. The note is written once per model, however
// often the agent reports it. It implements agent.Feedback and is called by the
// agent.
func (c *Chat) ModelInfoUnavailable() {
	c.mu.Lock()
	reported := c.model.reported
	c.model.reported = true
	c.mu.Unlock()

	if !reported {
		c.append(command.Error, "Model info unavailable; automatic context compaction is disabled.")
	}
}

// SessionReset implements agent.Feedback. The core needs no action here.
func (c *Chat) SessionReset() {}

// SessionStarted implements agent.Feedback. The core needs no action here.
func (c *Chat) SessionStarted() {}

// SessionClosed implements agent.Feedback. The core needs no action here.
func (c *Chat) SessionClosed() {}
