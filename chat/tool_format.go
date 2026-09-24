package chat

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jjmrocha/ai-chat/internal/format"
	"github.com/jjmrocha/go-algo/fn"
)

const (
	maxToolArgLen    = 200
	maxToolResultLen = 400
)

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

func formatToolResult(result string, err error, elapsed time.Duration) string {
	line := toolOutcome(result, err, elapsed)
	if elapsed > 0 {
		line += " · " + format.Duration(elapsed)
	}

	return line
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
