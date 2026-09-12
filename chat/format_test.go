package chat

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/stretchr/testify/assert"
)

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{name: "zero", input: 0, expected: "0"},
		{name: "under 1K", input: 500, expected: "500"},
		{name: "exactly 1K", input: 1000, expected: "1K"},
		{name: "1.5K", input: 1500, expected: "1.50K"},
		{name: "9.99K", input: 9990, expected: "9.99K"},
		{name: "exactly 1M", input: 1000000, expected: "1M"},
		{name: "2.5M", input: 2500000, expected: "2.50M"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			input := tc.input

			// when
			result := formatTokens(input)

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestDefaultTelemetryFormatter(t *testing.T) {
	t.Run("empty metadata", func(t *testing.T) {
		// given

		// when
		result := defaultTelemetryFormatter(agent.Metadata{})

		// then
		assert.Empty(t, result)
	})

	t.Run("tool calls only", func(t *testing.T) {
		// given
		meta := agent.Metadata{ToolCalls: 3}

		// when
		result := defaultTelemetryFormatter(meta)

		// then
		assert.Equal(t, " 3 tool calls", result)
	})

	t.Run("a single tool call reads as one call", func(t *testing.T) {
		// given
		meta := agent.Metadata{ToolCalls: 1}

		// when
		result := defaultTelemetryFormatter(meta)

		// then
		assert.Equal(t, " 1 tool call", result)
	})

	t.Run("reports sub-second work in milliseconds", func(t *testing.T) {
		// given
		meta := agent.Metadata{
			ToolCalls:    2,
			LLMDuration:  2200 * time.Millisecond,
			ToolDuration: 4 * time.Millisecond,
		}

		// when
		result := defaultTelemetryFormatter(meta)

		// then
		assert.Equal(t, " 2 tool calls · 2.2s llm · 4ms tools", result)
	})

	t.Run("all fields", func(t *testing.T) {
		// given
		meta := agent.Metadata{
			ToolCalls:    2,
			LLMDuration:  1300 * time.Millisecond,
			ToolDuration: 500 * time.Millisecond,
			PromptTokens: 1200,
			OutputTokens: 412,
			TotalTokens:  1500,
		}

		// when
		result := defaultTelemetryFormatter(meta)

		// then
		assert.Equal(t, " 2 tool calls · 1.3s llm · 500ms tools · ↑1.20K ↓412 tokens", result)
	})

	t.Run("input tokens only", func(t *testing.T) {
		// given
		meta := agent.Metadata{PromptTokens: 1200}

		// when
		result := defaultTelemetryFormatter(meta)

		// then
		assert.Equal(t, " ↑1.20K tokens", result)
	})

	t.Run("output tokens only", func(t *testing.T) {
		// given
		meta := agent.Metadata{OutputTokens: 100}

		// when
		result := defaultTelemetryFormatter(meta)

		// then
		assert.Equal(t, " ↓100 tokens", result)
	})

	t.Run("truncated reply flagged", func(t *testing.T) {
		tests := []struct {
			name       string
			stopReason string
		}{
			{name: "anthropic max_tokens", stopReason: "max_tokens"},
			{name: "openrouter length", stopReason: "length"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				// given
				meta := agent.Metadata{OutputTokens: 100, StopReason: tc.stopReason}

				// when
				result := defaultTelemetryFormatter(meta)

				// then
				assert.Contains(t, result, "truncated")
			})
		}
	})

	t.Run("normal stop reason not flagged", func(t *testing.T) {
		// given
		meta := agent.Metadata{OutputTokens: 100, StopReason: "end_turn"}

		// when
		result := defaultTelemetryFormatter(meta)

		// then
		assert.Equal(t, " ↓100 tokens", result)
	})
}

func TestDefaultStatusFormatter(t *testing.T) {
	t.Run("full info", func(t *testing.T) {
		// given
		info := StatusInfo{
			Name:     "gpt-4",
			Provider: "openai",
			Effort:   llm.EffortMedium,
			CtxPct:   12.5,
			Tokens:   8400,
		}

		// when
		result := defaultStatusFormatter(info)

		// then
		assert.Equal(t, "gpt-4 (openai) · medium · ctx: 12% · tokens: 8.40K", result)
	})

	t.Run("no name or provider", func(t *testing.T) {
		// given
		info := StatusInfo{Effort: llm.EffortOff}

		// when
		result := defaultStatusFormatter(info)

		// then
		assert.NotEmpty(t, result)
	})

	t.Run("effort off omitted", func(t *testing.T) {
		// given
		info := StatusInfo{
			Name:   "claude-3",
			Effort: llm.EffortOff,
			CtxPct: 50,
			Tokens: 500,
		}

		// when
		result := defaultStatusFormatter(info)

		// then
		assert.NotContains(t, result, "off")
	})

	t.Run("unknown provider", func(t *testing.T) {
		// given
		info := StatusInfo{
			Name:   "my-model",
			Effort: llm.EffortLow,
		}

		// when
		result := defaultStatusFormatter(info)

		// then
		assert.NotContains(t, result, "()")
	})
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
			expected: "  ⎿ ✗ no such file · 100ms",
		},
		{
			name:     "keeps only the error's first line",
			err:      errors.New("no such file\nstack trace here"),
			elapsed:  100 * time.Millisecond,
			expected: "  ⎿ ✗ no such file · 100ms",
		},
		{
			name:     "truncates a long error",
			err:      errors.New(long),
			elapsed:  100 * time.Millisecond,
			expected: "  ⎿ ✗ " + strings.Repeat("x", maxToolResultLen) + "… · 100ms",
		},
		{
			name:     "marks an empty result",
			elapsed:  100 * time.Millisecond,
			expected: "  ⎿ (empty) · 100ms",
		},
		{
			name:     "shows a single line that fits, unquoted",
			result:   "/Users/jrocha/SOURCES/GO/ai-chat",
			elapsed:  100 * time.Millisecond,
			expected: "  ⎿ /Users/jrocha/SOURCES/GO/ai-chat · 100ms",
		},
		{
			name:     "reports the size of a single line that does not fit",
			result:   long,
			elapsed:  300 * time.Millisecond,
			expected: "  ⎿ <401 B> · 300ms",
		},
		{
			name:     "reports the size of a multi-line result even when it is small",
			result:   "ok\nfine",
			elapsed:  300 * time.Millisecond,
			expected: "  ⎿ <7 B> · 300ms",
		},
		{
			name:     "reports a sub-second call in milliseconds",
			result:   "ok",
			elapsed:  50 * time.Millisecond,
			expected: "  ⎿ ok · 50ms",
		},
		{
			name:     "reports a very fast call rather than rounding it to nothing",
			result:   "ok",
			elapsed:  200 * time.Microsecond,
			expected: "  ⎿ ok · <1ms",
		},
		{
			name:     "reports a long call in seconds",
			result:   "ok",
			elapsed:  12 * time.Second,
			expected: "  ⎿ ok · 12.0s",
		},
		{
			name:     "reports the size of a result carrying control characters",
			result:   "ok\x1b[2Jgone",
			elapsed:  300 * time.Millisecond,
			expected: "  ⎿ <10 B> · 300ms",
		},
		{
			name:     "strips control characters from an error",
			err:      errors.New("refused \x1b[2J\x07now"),
			elapsed:  300 * time.Millisecond,
			expected: "  ⎿ ✗ refused [2Jnow · 300ms",
		},
		{
			name:     "marks a call that never returned",
			expected: "  ⎿ (no result)",
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
