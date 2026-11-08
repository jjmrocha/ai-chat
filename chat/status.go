package chat

import (
	"fmt"
	"strings"

	"github.com/jjmrocha/ai-toolkit/llm"
)

// StatusFormatter renders the status line a front-end shows below the input.
// Install one with [WithStatusFormatter].
type StatusFormatter func(StatusInfo) string

// Status returns the current model, effort and context usage. It reads the
// model and effort from the agent's model client, so it waits while a model
// change is in progress.
func (c *Chat) Status() StatusInfo {
	mi := c.model.ModelInfo()
	effort := c.model.Effort()

	c.mu.Lock()
	tokens := c.lastMeta.TotalTokens
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

// StatusText returns [Chat.Status] rendered by the status formatter.
func (c *Chat) StatusText() string { return c.statusFmt(c.Status()) }

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
