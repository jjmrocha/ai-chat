package chat

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/go-algo/fn"
)

var _ agent.Feedback = (*Chat)(nil)

const maxToolArgLen = 200

// ToolCalled records that the agent started a tool call, making it visible
// through [Chat.PendingTool]. It implements agent.Feedback and is called by the
// agent, not by your code.
func (c *Chat) ToolCalled(name string, args map[string]any) {
	c.mu.Lock()
	c.pendingTool = formatToolCall(name, args)
	c.mu.Unlock()
	c.notify()
}

func formatToolCall(name string, args map[string]any) string {
	argNames := slices.Sorted(maps.Keys(args))

	parts := fn.Map(argNames, func(argName string) string {
		return formatToolArg(argName, args[argName])
	})

	return name + "(" + strings.Join(parts, ", ") + ")"
}

func formatToolArg(name string, value any) string {
	return name + "=" + formatValue(value, maxToolArgLen)
}

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

// ToolReturned records the outcome of the tool call in flight, appending it as
// one [command.Activity] line whose Text is the call and whose Detail is the
// result. It implements agent.Feedback and is called by the agent.
func (c *Chat) ToolReturned(_ string, result string, err error, elapsed time.Duration) {
	c.closeToolCall(formatToolResult(result, err, elapsed))
}

func (c *Chat) closeToolCall(response string) {
	c.mu.Lock()
	request := c.pendingTool
	c.pendingTool = ""
	c.mu.Unlock()

	if request == "" {
		return
	}

	c.appendLine(Line{Kind: command.Activity, Text: request, Detail: response})
}

// ContextCompacted notes a successful context compaction in the transcript. It
// implements agent.Feedback and is called by the agent.
func (c *Chat) ContextCompacted() { c.append(command.Info, "Context compacted.") }

// ContextCompactionFailed notes a failed context compaction in the transcript.
// It implements agent.Feedback and is called by the agent.
func (c *Chat) ContextCompactionFailed() {
	c.append(command.Error, "Context compaction failed; will retry after the next turn.")
}

// ModelInfoUnavailable notes that the model's limits could not be read, which
// disables automatic compaction. It implements agent.Feedback and is called by
// the agent.
func (c *Chat) ModelInfoUnavailable() {
	c.append(command.Error, "Model info unavailable; automatic context compaction is disabled.")
}

// SessionReset implements agent.Feedback. The core needs no action here.
func (c *Chat) SessionReset() {}

// SessionStarted implements agent.Feedback. The core needs no action here.
func (c *Chat) SessionStarted() {}

// SessionClosed implements agent.Feedback. The core needs no action here.
func (c *Chat) SessionClosed() {}
