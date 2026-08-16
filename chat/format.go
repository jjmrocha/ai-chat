package chat

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
)

// StatusInfo is the data a StatusFormatter renders into the bottom status bar.
type StatusInfo struct {
	Name     string
	Provider llm.Provider
	Effort   llm.Effort
	CtxPct   float64
	Tokens   int
}

// TelemetryFormatter renders a turn's usage/timing into the plain-text line
// appended after a reply. The UI applies color.
type TelemetryFormatter func(agent.Metadata) string

// StatusFormatter renders the current status into the plain-text bottom bar.
// The UI applies color.
type StatusFormatter func(StatusInfo) string

// Status assembles the current status data from the agent and last turn.
func (c *Chat) Status() StatusInfo {
	meta := c.LastMetadata()
	info := StatusInfo{Tokens: meta.TotalTokens}
	if mi := c.agent.ModelInfo(c.ctx); mi != nil {
		info.Name = mi.ModelName
		info.Provider = mi.Provider
		info.Effort = mi.Effort
		if mi.ModelContextSize > 0 {
			info.CtxPct = float64(meta.TotalTokens) * 100 / float64(mi.ModelContextSize)
		}
	}
	return info
}

// StatusText renders the status bar as plain text via the status formatter.
func (c *Chat) StatusText() string { return c.statusFmt(c.Status()) }

func defaultTelemetryFormatter(meta agent.Metadata) string {
	var parts []string
	if meta.ToolCalls > 0 {
		parts = append(parts, fmt.Sprintf("%d tool calls", meta.ToolCalls))
	}
	if meta.LLMDuration > 0 {
		parts = append(parts, fmt.Sprintf("%.1fs llm", meta.LLMDuration.Seconds()))
	}
	if meta.ToolDuration > 0 {
		parts = append(parts, fmt.Sprintf("%.1fs tools", meta.ToolDuration.Seconds()))
	}
	if meta.OutputTokens > 0 {
		parts = append(parts, fmt.Sprintf("%d out tok", meta.OutputTokens))
	}
	if truncated(meta.StopReason) {
		parts = append(parts, "⚠ truncated")
	}
	if len(parts) == 0 {
		return ""
	}
	return "[" + strings.Join(parts, " · ") + "]"
}

// truncated reports whether the provider stopped the reply at its output-token
// limit: "max_tokens" is Anthropic's value, "length" OpenRouter's.
func truncated(stopReason string) bool {
	return stopReason == "max_tokens" || stopReason == "length"
}

func defaultStatusFormatter(info StatusInfo) string {
	name := info.Name
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
	parts = append(parts, fmt.Sprintf("ctx:%.0f%%", info.CtxPct))
	parts = append(parts, fmt.Sprintf("%s tok", formatTokens(info.Tokens)))
	return strings.Join(parts, " · ")
}

func formatTokens(tokens int) string {
	switch {
	case tokens >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(tokens)/1_000_000)
	case tokens >= 1_000:
		return fmt.Sprintf("%.2fK", float64(tokens)/1_000)
	default:
		return strconv.Itoa(tokens)
	}
}
