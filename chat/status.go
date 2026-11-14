package chat

import (
	"fmt"
	"strings"

	"github.com/jjmrocha/ai-toolkit/llm"
)

// StatusFormatter renders the status line a front-end shows below the input.
// Install one with [WithStatusFormatter].
type StatusFormatter func(StatusInfo) string

// StatusText returns the status line: the active model and provider, the
// reasoning effort and how much of the context window the last turn used,
// rendered by the status formatter. It never waits on the model client, not
// even while /model is switching models, so a front-end may call it on every
// frame.
func (c *Chat) StatusText() string { return c.statusFmt(c.status()) }

func (c *Chat) status() StatusInfo {
	c.mu.Lock()
	mi, effort, tokens := c.modelInfo, c.effort, c.lastMeta.TotalTokens
	c.mu.Unlock()

	info := StatusInfo{
		Model:    sanitize(mi.Name),
		Provider: mi.Provider,
		Effort:   effort,
		Tokens:   tokens,
	}
	if mi.ContextSize > 0 {
		info.ContextPercent = float64(tokens) * 100 / float64(mi.ContextSize)
	}
	return info
}

func (c *Chat) refreshModel() {
	info, effort := *c.model.ModelInfo(), c.model.Effort()
	c.mutate(func() { c.modelInfo, c.effort = info, effort })
}

func defaultStatusFormatter(info StatusInfo) string {
	name := info.Model
	if name == "" {
		name = "—"
	}
	if info.Provider != "" {
		name = fmt.Sprintf("%s (%s)", name, info.Provider)
	}
	parts := []string{name}
	if info.Effort != llm.EffortOff && info.Effort != "" {
		parts = append(parts, string(info.Effort))
	}
	parts = append(parts, fmt.Sprintf("ctx: %.0f%%", info.ContextPercent))
	parts = append(parts, fmt.Sprintf("tokens: %s", formatTokens(info.Tokens)))
	return strings.Join(parts, " · ")
}
