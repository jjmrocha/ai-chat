package chat

import "github.com/jjmrocha/ai-toolkit/subagent"

var _ subagent.Feedback = subagentFeedback{}

type subagentState struct {
	running bool
	tools   int
	tokens  int
	cached  int
}

func (s subagentState) text() string {
	if !s.running {
		return ""
	}
	text := plural(s.tools, "tool") + " · " + formatTokens(s.tokens) + " tokens"
	if s.cached > 0 {
		text += " (" + formatTokens(s.cached) + " cached)"
	}
	return text
}

type subagentFeedback struct{ c *Chat }

func (f subagentFeedback) SubagentStarted(_ string) {
	f.c.mutate(func() { f.c.subagent = subagentState{running: true} })
}

func (f subagentFeedback) SubagentFinished(_ string, _ error) {
	f.c.mutate(func() { f.c.subagent = subagentState{} })
}

func (f subagentFeedback) ToolCalled(_ string, _ string) {
	f.c.mutate(func() { f.c.subagent.tools++ })
}

func (f subagentFeedback) TokensUsed(_ string, totalTokens, cacheReadTokens int) {
	f.c.mutate(func() {
		f.c.subagent.tokens = totalTokens
		f.c.subagent.cached = cacheReadTokens
	})
}
