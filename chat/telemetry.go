package chat

import (
	"fmt"
	"strings"

	"github.com/jjmrocha/ai-chat/internal/format"
	"github.com/jjmrocha/ai-toolkit/agent"
)

// TelemetryFormatter renders the telemetry line appended after a turn. Return
// an empty string to append nothing. Install one with [WithTelemetryFormatter].
type TelemetryFormatter func(agent.Metadata) string

func defaultTelemetryFormatter(meta agent.Metadata) string {
	var parts []string
	if meta.ToolCalls > 0 {
		parts = append(parts, plural(meta.ToolCalls, "tool call"))
	}
	if meta.LLMDuration > 0 {
		parts = append(parts, format.Duration(meta.LLMDuration)+" llm")
	}
	if meta.ToolDuration > 0 {
		parts = append(parts, format.Duration(meta.ToolDuration)+" tools")
	}
	if tok := tokenPart(meta); tok != "" {
		parts = append(parts, tok)
	}
	if truncated(meta.StopReason) {
		parts = append(parts, "⚠ truncated")
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " · ")
}

func tokenPart(meta agent.Metadata) string {
	var sides []string
	if meta.PromptTokens > 0 {
		sides = append(sides, "↑"+formatTokens(meta.PromptTokens)+cachedPart(meta))
	}
	if meta.OutputTokens > 0 {
		sides = append(sides, "↓"+formatTokens(meta.OutputTokens))
	}
	if len(sides) == 0 {
		return ""
	}
	return strings.Join(sides, " ") + " tokens"
}

func cachedPart(meta agent.Metadata) string {
	percent := meta.CacheReadTokens * 100 / meta.PromptTokens
	if percent == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d%% cached)", percent)
}

func truncated(stopReason string) bool {
	return stopReason == "max_tokens" || stopReason == "length"
}
