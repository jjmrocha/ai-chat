package chat

import (
	"slices"
	"strings"
	"time"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
)

var _ agent.Feedback = agentFeedback{}

type agentFeedback struct{ c *Chat }

func (f agentFeedback) ToolCalled(name string, args map[string]any) {
	f.c.mutate(func() {
		f.c.pendingTool = sanitize(formatToolCall(name, args))
		f.c.stageStart = f.c.now()
	})
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

func (f agentFeedback) TokensUsed(totalTokens, _ int) { f.c.setTokens(totalTokens) }

func (f agentFeedback) ContextCompacted() { f.c.append(Info, "Context compacted.") }

func (f agentFeedback) ContextCompactionFailed() {
	if cancelledByUser(f.c.workContext()) {
		return
	}
	f.c.append(Error, "Context compaction failed; will retry after the next turn.")
}

func (f agentFeedback) SessionReset() {}

func (f agentFeedback) SessionStarted() {}

func (f agentFeedback) SessionResumed(sessionID string) {
	messages := f.c.agent.Messages()
	for _, msg := range messages {
		f.c.replay(msg)
	}

	f.c.setTokens(lastTotalTokens(messages))
	f.c.append(Info, "Session "+sessionID+" resumed.")
}

func (f agentFeedback) SessionClosed() {}

func (c *Chat) closeToolCall(response string) {
	c.mu.Lock()
	request := c.pendingTool
	c.pendingTool = ""
	if request != "" {
		c.stageStart = c.now()
	}
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

func lastTotalTokens(messages []llm.Message) int {
	for _, msg := range slices.Backward(messages) {
		if m, ok := msg.(llm.AssistantMessage); ok {
			return m.Stats.TotalTokens
		}
	}
	return 0
}
