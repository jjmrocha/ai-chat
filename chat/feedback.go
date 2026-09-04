package chat

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/agent"
)

var _ agent.Feedback = (*Chat)(nil)

const maxToolArgLen = 200

// ToolCalled implements agent.Feedback.
func (c *Chat) ToolCalled(name string, args map[string]any) {
	c.append(command.Activity, formatToolCall(name, args))
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

	return "● " + name + "(" + strings.Join(parts, ", ") + ")"
}

// formatToolArg renders one argument, eliding anything that would not fit on
// the activity line: a long string, an object or a list.
func formatToolArg(name string, value any) string {
	switch v := value.(type) {
	case nil:
		return name + "=null"
	case string:
		if len(v) > maxToolArgLen {
			return name + ": ..."
		}

		return name + "=" + strconv.Quote(v)
	case bool, float64, int:
		return name + "=" + fmt.Sprint(v)
	default:
		return name + ": ..."
	}
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
