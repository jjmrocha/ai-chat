package chat

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
	if mi := c.agent.ModelInfo(c.baseCtx); mi != nil {
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
		parts = append(parts, plural(meta.ToolCalls, "tool call"))
	}
	if meta.LLMDuration > 0 {
		parts = append(parts, formatDuration(meta.LLMDuration)+" llm")
	}
	if meta.ToolDuration > 0 {
		parts = append(parts, formatDuration(meta.ToolDuration)+" tools")
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

// tokenPart renders the turn's input and output counts as "↑in ↓out tokens",
// dropping whichever side the provider did not report.
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
	parts = append(parts, fmt.Sprintf("ctx: %.0f%%", info.CtxPct))
	parts = append(parts, fmt.Sprintf("tokens: %s", formatTokens(info.Tokens)))
	return strings.Join(parts, " · ")
}

func formatTokens(tokens int) string {
	switch {
	case tokens >= 1_000_000:
		v := float64(tokens) / 1_000_000
		if v == float64(int(v)) {
			return fmt.Sprintf("%dM", tokens/1_000_000)
		}
		return fmt.Sprintf("%.2fM", v)
	case tokens >= 1_000:
		v := float64(tokens) / 1_000
		if v == float64(int(v)) {
			return fmt.Sprintf("%dK", tokens/1_000)
		}
		return fmt.Sprintf("%.2fK", v)
	default:
		return strconv.Itoa(tokens)
	}
}

// formatBytes renders a size for display, in the largest unit that leaves a
// whole part.
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
	// maxToolResultLen caps the result text shown on a response line. It is
	// larger than maxToolArgLen because the result owns a whole line, where an
	// argument shares one with the rest of the call.
	maxToolResultLen = 400
)

// formatToolResult renders the response line that closes a tool call: what the
// call produced, and how long it took. A zero elapsed with no result and no
// error is a call that never returned.
func formatToolResult(result string, err error, elapsed time.Duration) string {
	line := "  ⎿ " + toolOutcome(result, err, elapsed)
	if elapsed > 0 {
		line += " · " + formatDuration(elapsed)
	}

	return line
}

// formatDuration renders how long a call took, in whole milliseconds below a
// second. Local tools routinely finish in a few of them, and reporting those as
// "0.0s" would hide the timing this line exists to show.
func formatDuration(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.1fs", d.Seconds())
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

// formatOutput renders a tool's output: the text itself when it is a single
// line within budget, and its size otherwise. It prints bare rather than
// quoted, because output is not a literal. A multi-line result always reports
// its size — the response is one line, and the tools return tagged text whose
// shape this code has no business interpreting.
func formatOutput(result string) string {
	if len(result) > maxToolResultLen || strings.ContainsFunc(result, isControl) {
		return formatBytes(len(result))
	}

	return result
}

// isControl reports whether r steers the terminal rather than printing on it.
// Tool output is untrusted — a shell command's output, or whatever an MCP
// server chose to send — and it reaches the terminal unescaped, so an escape
// sequence left in it would move the cursor, clear the screen, or corrupt the
// live region. Newline counts: a result spanning lines does not belong on one.
func isControl(r rune) bool {
	return r < 0x20 || r == 0x7f
}

// stripControl removes the characters isControl rejects, for text that is shown
// rather than measured.
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

	return s[:budget] + "…"
}
