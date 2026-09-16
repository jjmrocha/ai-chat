package chat

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
)

// StatusInfo is the state a status line is built from: the active model and
// provider, the reasoning effort, how full the context window is as a
// percentage, and the token total of the last turn. Fields are zero when the
// model's limits are not known yet.
type StatusInfo struct {
	Name     string
	Provider llm.Provider
	Effort   llm.Effort
	CtxPct   float64
	Tokens   int
}

// TelemetryFormatter renders the telemetry line appended after a turn. Return
// an empty string to append nothing. Install one with [WithTelemetryFormatter].
type TelemetryFormatter func(agent.Metadata) string

// StatusFormatter renders the status line a front-end shows below the input.
// Install one with [WithStatusFormatter].
type StatusFormatter func(StatusInfo) string

// Status returns the current model, effort and context usage. The result is
// cached and recomputed after a turn, a model or effort change, or a clear, so
// a front-end may call it on every frame.
func (c *Chat) Status() StatusInfo {
	c.mu.Lock()
	if cached := c.statusCache; cached != nil {
		c.mu.Unlock()
		return *cached
	}
	meta := c.lastMeta
	c.mu.Unlock()

	info := StatusInfo{Tokens: meta.TotalTokens}
	mi := c.agent.ModelInfo(c.baseCtx)
	if mi != nil {
		info.Name = mi.ModelName
		info.Provider = mi.Provider
		info.Effort = mi.Effort
		if mi.ModelContextSize > 0 {
			info.CtxPct = float64(meta.TotalTokens) * 100 / float64(mi.ModelContextSize)
		}

		c.mu.Lock()
		c.statusCache = &info
		c.mu.Unlock()
	}

	return info
}

func (c *Chat) invalidateStatus() {
	c.mu.Lock()
	c.statusCache = nil
	c.mu.Unlock()
}

// StatusText returns [Chat.Status] rendered by the status formatter.
func (c *Chat) StatusText() string { return c.statusFmt(c.Status()) }

func defaultTelemetryFormatter(meta agent.Metadata) string {
	var parts []string
	if meta.ToolCalls > 0 {
		parts = append(parts, plural(meta.ToolCalls, "tool call"))
	}
	if meta.LLMDuration > 0 {
		parts = append(parts, FormatDuration(meta.LLMDuration)+" llm")
	}
	if meta.ToolDuration > 0 {
		parts = append(parts, FormatDuration(meta.ToolDuration)+" tools")
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
		sides = append(sides, "↑"+formatTokens(meta.PromptTokens))
	}
	if meta.OutputTokens > 0 {
		sides = append(sides, "↓"+formatTokens(meta.OutputTokens))
	}
	if len(sides) == 0 {
		return ""
	}
	return strings.Join(sides, " ") + " tokens"
}

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
	parts = append(parts, fmt.Sprintf("ctx: %.0f%%", info.CtxPct))
	parts = append(parts, fmt.Sprintf("tokens: %s", formatTokens(info.Tokens)))
	return strings.Join(parts, " · ")
}

func formatTokens(tokens int) string {
	switch {
	case tokens >= 1_000_000:
		return scaleSuffix(tokens, 1_000_000, "M")
	case tokens >= 1_000:
		return scaleSuffix(tokens, 1_000, "K")
	default:
		return strconv.Itoa(tokens)
	}
}

func scaleSuffix(n, unit int, suffix string) string {
	v := float64(n) / float64(unit)
	if v == float64(int(v)) {
		return strconv.Itoa(n/unit) + suffix
	}
	return fmt.Sprintf("%.2f%s", v, suffix)
}

func formatBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("<%.1f MB>", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("<%.1f KB>", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("<%d B>", n)
	}
}

const (
	maxToolResultLen = 400
)

func formatToolResult(result string, err error, elapsed time.Duration) string {
	line := toolOutcome(result, err, elapsed)
	if elapsed > 0 {
		line += " · " + FormatDuration(elapsed)
	}

	return line
}

// FormatDuration renders d the way the transcript and the front-end's progress
// line both show elapsed time: whole seconds and above in Go's own notation
// ("2m2s"), shorter spans in milliseconds ("340ms", "<1ms").
func FormatDuration(d time.Duration) string {
	switch {
	case d >= time.Second:
		return d.Truncate(time.Second).String()
	case d >= time.Millisecond:
		return strconv.FormatInt(d.Milliseconds(), 10) + "ms"
	default:
		return "<1ms"
	}
}

func toolOutcome(result string, err error, elapsed time.Duration) string {
	switch {
	case err != nil:
		return "✗ " + truncate(stripControl(firstLine(err.Error())), maxToolResultLen)
	case result != "":
		return formatOutput(result)
	case elapsed == 0:
		return "(no result)"
	default:
		return "(empty)"
	}
}

func formatOutput(result string) string {
	if len(result) > maxToolResultLen || strings.ContainsFunc(result, isControl) {
		return formatBytes(len(result))
	}

	return result
}

func isControl(r rune) bool {
	return r < 0x20 || r == 0x7f
}

func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if isControl(r) {
			return -1
		}

		return r
	}, s)
}

func firstLine(s string) string {
	head, _, _ := strings.Cut(s, "\n")

	return head
}

func truncate(s string, budget int) string {
	if len(s) <= budget {
		return s
	}

	cut := budget
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}

	return s[:cut] + "…"
}
