package chat

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/agent"
)

var _ agent.Feedback = (*Chat)(nil)

const maxToolArgLen = 200

// ToolCalled implements agent.Feedback. The call is held rather than printed:
// a finished transcript line cannot be revisited, so the request waits in the
// live region for the result that completes it.
func (c *Chat) ToolCalled(name string, args map[string]any) {
	c.mu.Lock()
	c.pendingTool = formatToolCall(name, args)
	c.mu.Unlock()
	c.notify()
}

func formatToolCall(name string, args map[string]any) string {
	argNames := make([]string, 0, len(args))
	for argName := range args {
		argNames = append(argNames, argName)
	}

	slices.Sort(argNames)

	parts := make([]string, 0, len(argNames))
	for _, argName := range argNames {
		parts = append(parts, formatToolArg(argName, args[argName]))
	}

	return name + "(" + strings.Join(parts, ", ") + ")"
}

func formatToolArg(name string, value any) string {
	return name + "=" + formatValue(value, maxToolArgLen)
}

// formatValue renders one value for display: the value itself when it fits
// within budget, and its size or its shape when it does not. Arguments arrive
// decoded from JSON, so numbers are float64 and the composite types are
// map[string]any and []any.
func formatValue(value any, budget int) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		if len(v) > budget {
			return formatBytes(len(v))
		}

		return strconv.Quote(v)
	case bool, float64, int:
		return fmt.Sprint(v)
	case map[string]any:
		return encodeOrShape(v, budget, "{"+plural(len(v), "key")+"}")
	case []any:
		return encodeOrShape(v, budget, "["+strconv.Itoa(len(v))+"]")
	default:
		return encodeOrShape(v, budget, "<?>")
	}
}

// encodeOrShape renders value as compact JSON, falling back to shape when the
// encoding would not fit within budget or cannot be produced at all.
func encodeOrShape(value any, budget int, shape string) string {
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) > budget {
		return shape
	}

	return string(encoded)
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}

	return strconv.Itoa(n) + " " + noun + "s"
}

// ToolReturned implements agent.Feedback.
func (c *Chat) ToolReturned(_ string, result string, err error, elapsed time.Duration) {
	c.closeToolCall(formatToolResult(result, err, elapsed))
}

// closeToolCall prints the call in flight together with the response line that
// closes it, as one transcript entry: the UI opens a block with a blank line,
// so a request and its result split across two entries would be pulled apart.
// It does nothing when no call is in flight.
func (c *Chat) closeToolCall(response string) {
	c.mu.Lock()
	request := c.pendingTool
	c.pendingTool = ""
	c.mu.Unlock()

	if request == "" {
		return
	}

	c.append(command.Activity, "● "+request+"\n"+response)
}

// ContextCompacted implements agent.Feedback.
func (c *Chat) ContextCompacted() { c.append(command.Activity, "● context compacted") }

// ContextCompactionFailed implements agent.Feedback.
func (c *Chat) ContextCompactionFailed() {
	c.append(command.Error, "Context compaction failed; will retry after the next turn.")
}

// ModelInfoUnavailable implements agent.Feedback.
func (c *Chat) ModelInfoUnavailable() {
	c.append(command.Error, "Model info unavailable; automatic context compaction is disabled.")
}

func (c *Chat) SessionReset()   {}
func (c *Chat) SessionStarted() {}
func (c *Chat) SessionClosed()  {}
