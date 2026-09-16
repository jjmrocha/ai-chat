package chat

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		assert.Equal(t, " 2 tool calls · 2s llm · 4ms tools", result)
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
		assert.Equal(t, " 2 tool calls · 1s llm · 500ms tools · ↑1.20K ↓412 tokens", result)
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

func TestFormatDuration(t *testing.T) {
	testCases := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{name: "under a millisecond", input: 400 * time.Microsecond, expected: "<1ms"},
		{name: "milliseconds", input: 340 * time.Millisecond, expected: "340ms"},
		{name: "exactly a second", input: time.Second, expected: "1s"},
		{name: "drops the fraction of a second", input: 1700 * time.Millisecond, expected: "1s"},
		{name: "minutes and seconds", input: 122 * time.Second, expected: "2m2s"},
		{name: "whole minutes", input: 2 * time.Minute, expected: "2m0s"},
		{name: "hours", input: time.Hour + 61*time.Second, expected: "1h1m1s"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			input := tc.input

			// when
			result := FormatDuration(input)

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

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		budget   int
		expected string
	}{
		{name: "under budget is untouched", input: "abc", budget: 5, expected: "abc"},
		{name: "exactly at budget is untouched", input: "abcde", budget: 5, expected: "abcde"},
		{name: "over budget gains an ellipsis", input: "abcdef", budget: 5, expected: "abcde…"},
		{name: "zero budget keeps only the ellipsis", input: "abc", budget: 0, expected: "…"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			input, budget := tc.input, tc.budget

			// when
			result := truncate(input, budget)

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTruncateNeverSplitsARune(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		budget int
	}{
		{name: "cut inside a 2-byte rune", input: strings.Repeat("é", 10), budget: 5},
		{name: "cut inside a 3-byte rune", input: strings.Repeat("€", 10), budget: 5},
		{name: "cut inside a 4-byte emoji", input: strings.Repeat("😀", 10), budget: 5},
		{name: "cut just after a multi-byte rune", input: "é" + strings.Repeat("a", 10), budget: 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			input, budget := tc.input, tc.budget

			// when
			result := truncate(input, budget)

			// then
			assert.True(t, utf8.ValidString(result), "produced invalid UTF-8: %q", result)
			assert.True(t, strings.HasSuffix(result, "…"))
			assert.True(t, strings.HasPrefix(tc.input, strings.TrimSuffix(result, "…")))
		})
	}
}

func TestStatusUsesModelInfo(t *testing.T) {
	// given
	backend := &mockedAgentBackend{
		modelInfoFunc: func(context.Context) *agent.ModelInfo {
			return &agent.ModelInfo{
				ModelName:        "m1",
				Provider:         llm.ProviderOllama,
				Effort:           llm.EffortLow,
				ModelContextSize: 1000,
			}
		},
	}
	c, _ := newTestChat(t, backend)
	c.mu.Lock()
	c.lastMeta = agent.Metadata{TotalTokens: 250}
	c.mu.Unlock()

	// when
	result := c.Status()

	// then
	assert.Equal(t, "m1", result.Name)
	assert.Equal(t, llm.ProviderOllama, result.Provider)
	assert.Equal(t, llm.EffortLow, result.Effort)
	assert.Equal(t, 250, result.Tokens)
	assert.InDelta(t, 25.0, result.CtxPct, 0.001)
}

func TestStatusIsCachedUntilInvalidated(t *testing.T) {
	// given
	backend := &mockedAgentBackend{
		modelInfoFunc: func(context.Context) *agent.ModelInfo {
			return &agent.ModelInfo{ModelName: "m1", ModelContextSize: 1000}
		},
	}
	c, _ := newTestChat(t, backend)

	// when
	for i := 0; i < 10; i++ {
		c.Status()
	}

	// then
	assert.Equal(t, 1, backend.infoHits())
}

func TestStatusCacheInvalidation(t *testing.T) {
	tests := []struct {
		name       string
		invalidate func(*Chat)
	}{
		{name: "changing model", invalidate: func(c *Chat) { _ = c.ChangeModel("m2") }},
		{name: "changing effort", invalidate: func(c *Chat) { _ = c.ChangeEffort(llm.EffortMax) }},
		{name: "clearing the session", invalidate: func(c *Chat) { _ = c.Clear() }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			backend := &mockedAgentBackend{
				modelInfoFunc: func(context.Context) *agent.ModelInfo {
					return &agent.ModelInfo{ModelName: "m1", ModelContextSize: 1000}
				},
			}
			c, _ := newTestChat(t, backend)
			c.Status()
			require.Equal(t, 1, backend.infoHits())

			// when
			tc.invalidate(c)
			c.Status()

			// then
			assert.Equal(t, 2, backend.infoHits())
		})
	}
}

func TestStatusInvalidatedAfterATurn(t *testing.T) {
	// given
	backend := &mockedAgentBackend{
		modelInfoFunc: func(context.Context) *agent.ModelInfo {
			return &agent.ModelInfo{ModelName: "m1", ModelContextSize: 1000}
		},
	}
	c, _ := newTestChat(t, backend)
	c.Status()
	require.Equal(t, 1, backend.infoHits())

	// when
	c.Submit("hi")
	waitIdle(t, c)
	c.Status()

	// then
	assert.Equal(t, 2, backend.infoHits())
}

func TestStatusDoesNotCacheAFailedLookup(t *testing.T) {
	// given
	backend := &mockedAgentBackend{
		modelInfoFunc: func(context.Context) *agent.ModelInfo { return nil },
	}
	c, _ := newTestChat(t, backend)

	// when
	c.Status()
	c.Status()
	c.Status()

	// then
	assert.Equal(t, 3, backend.infoHits())
}
