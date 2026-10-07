package chat

import (
	"strings"
	"time"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
)

var _ agent.Feedback = agentFeedback{}

type agentFeedback struct{ c *Chat }

func (f agentFeedback) ToolCalled(name string, args map[string]any) {
	f.c.mutate(func() { f.c.pendingTool = sanitize(formatToolCall(name, args)) })
}

func (f agentFeedback) ToolReturned(_ string, result string, err error, elapsed time.Duration) {
	f.c.closeToolCall(formatToolResult(result, err, elapsed))
}

func (f agentFeedback) InterimTextReceived(content string) {
	text := sanitize(content)
	if strings.TrimSpace(text) == "" {
		return
	}

	f.c.append(Reply, text)
}

func (f agentFeedback) TokensUsed(totalTokens int) { f.c.setTokens(totalTokens) }

func (f agentFeedback) ContextCompacted() { f.c.append(Info, "Context compacted.") }

func (f agentFeedback) ContextCompactionFailed() {
	if cancelledByUser(f.c.workContext()) {
		return
	}
	f.c.append(Error, "Context compaction failed; will retry after the next turn.")
}

func (f agentFeedback) ModelInfoUnavailable() {
	c := f.c
	c.mu.Lock()
	reported := c.model.reported
	c.model.reported = true
	c.mu.Unlock()

	if !reported {
		c.append(Error, "Model info unavailable; automatic context compaction is disabled.")
	}
}

func (f agentFeedback) SessionReset() {}

func (f agentFeedback) SessionStarted() {}

func (f agentFeedback) SessionResumed(sessionID string) {
	c := f.c
	tokens := 0
	for _, msg := range c.agent.Messages() {
		if m, ok := msg.(llm.AssistantMessage); ok {
			tokens = m.Stats.TotalTokens
		}
		c.replay(msg)
	}

	c.setTokens(tokens)
	c.append(Info, "Session "+sessionID+" resumed.")
}

func (f agentFeedback) SessionClosed() {}

func (c *Chat) closeToolCall(response string) {
	c.mu.Lock()
	request := c.pendingTool
	c.pendingTool = ""
	c.mu.Unlock()

	if request == "" {
		return
	}

	c.appendLine(Line{Kind: Activity, Text: request, Detail: response})
}

func (c *Chat) setTokens(totalTokens int) {
	c.mutate(func() { c.lastMeta.TotalTokens = totalTokens })
}

func (c *Chat) replay(msg llm.Message) {
	switch m := msg.(type) {
	case llm.UserMessage:
		c.append(User, m.Content)
	case llm.AssistantMessage:
		if len(m.ToolCalls) == 0 {
			c.append(Reply, m.Content)
		}
	}
}
