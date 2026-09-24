package chat

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatToolCall(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		args     map[string]any
		expected string
	}{
		{
			name:     "no arguments",
			tool:     "list",
			args:     nil,
			expected: "list()",
		},
		{
			name:     "arguments are sorted by name",
			tool:     "read",
			args:     map[string]any{"path": "a.go", "limit": 10},
			expected: `read(limit=10, path="a.go")`,
		},
		{
			name:     "nil value renders as null",
			tool:     "x",
			args:     map[string]any{"v": nil},
			expected: "x(v=null)",
		},
		{
			name:     "boolean value",
			tool:     "x",
			args:     map[string]any{"v": true},
			expected: "x(v=true)",
		},
		{
			name:     "oversized string collapses to a size",
			tool:     "x",
			args:     map[string]any{"v": strings.Repeat("a", maxToolArgLen+1)},
			expected: "x(v=<201 B>)",
		},
		{
			name:     "map collapses to a key count when oversized",
			tool:     "x",
			args:     map[string]any{"v": map[string]any{"k": strings.Repeat("b", maxToolArgLen+1)}},
			expected: "x(v={1 key})",
		},
		{
			name:     "small map is encoded",
			tool:     "x",
			args:     map[string]any{"v": map[string]any{"k": 1}},
			expected: `x(v={"k":1})`,
		},
		{
			name:     "slice collapses to a length when oversized",
			tool:     "x",
			args:     map[string]any{"v": []any{strings.Repeat("c", maxToolArgLen+1)}},
			expected: "x(v=[1])",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			tool, args := tc.tool, tc.args

			// when
			result := formatToolCall(tool, args)

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFormatToolResult(t *testing.T) {
	long := strings.Repeat("x", maxToolResultLen+1)

	testCases := []struct {
		name     string
		result   string
		err      error
		elapsed  time.Duration
		expected string
	}{
		{
			name:     "reports the error and not the result when the call failed",
			result:   "ignored",
			err:      errors.New("no such file"),
			elapsed:  100 * time.Millisecond,
			expected: "✗ no such file · 100ms",
		},
		{
			name:     "keeps only the error's first line",
			err:      errors.New("no such file\nstack trace here"),
			elapsed:  100 * time.Millisecond,
			expected: "✗ no such file · 100ms",
		},
		{
			name:     "truncates a long error",
			err:      errors.New(long),
			elapsed:  100 * time.Millisecond,
			expected: "✗ " + strings.Repeat("x", maxToolResultLen) + "… · 100ms",
		},
		{
			name:     "marks an empty result",
			elapsed:  100 * time.Millisecond,
			expected: "(empty) · 100ms",
		},
		{
			name:     "shows a single line that fits, unquoted",
			result:   "/Users/jrocha/SOURCES/GO/ai-chat",
			elapsed:  100 * time.Millisecond,
			expected: "/Users/jrocha/SOURCES/GO/ai-chat · 100ms",
		},
		{
			name:     "reports the size of a single line that does not fit",
			result:   long,
			elapsed:  300 * time.Millisecond,
			expected: "<401 B> · 300ms",
		},
		{
			name:     "reports the size of a multi-line result even when it is small",
			result:   "ok\nfine",
			elapsed:  300 * time.Millisecond,
			expected: "<7 B> · 300ms",
		},
		{
			name:     "reports a sub-second call in milliseconds",
			result:   "ok",
			elapsed:  50 * time.Millisecond,
			expected: "ok · 50ms",
		},
		{
			name:     "reports a very fast call rather than rounding it to nothing",
			result:   "ok",
			elapsed:  200 * time.Microsecond,
			expected: "ok · <1ms",
		},
		{
			name:     "reports a long call in seconds",
			result:   "ok",
			elapsed:  12 * time.Second,
			expected: "ok · 12s",
		},
		{
			name:     "reports the size of a result carrying control characters",
			result:   "ok\x1b[2Jgone",
			elapsed:  300 * time.Millisecond,
			expected: "<10 B> · 300ms",
		},
		{
			name:     "strips control characters from an error",
			err:      errors.New("refused \x1b[2J\x07now"),
			elapsed:  300 * time.Millisecond,
			expected: "✗ refused [2Jnow · 300ms",
		},
		{
			name:     "marks a call that never returned",
			expected: "(no result)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			result := formatToolResult(tc.result, tc.err, tc.elapsed)
			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}
