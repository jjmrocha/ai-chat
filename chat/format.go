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

// defaultTelemetryFormatter reproduces the built-in per-turn telemetry line,
// e.g. "[2 tool calls · 1.3s llm · 412 out tok]". Empty when nothing to report.
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
	if len(parts) == 0 {
		return ""
	}
	return "[" + strings.Join(parts, " · ") + "]"
}

// defaultStatusFormatter reproduces the built-in status bar, e.g.
// "model (provider) · medium · ctx:12% · 8.40K tok".
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
