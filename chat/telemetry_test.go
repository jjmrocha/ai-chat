package chat

import (
	"testing"
	"time"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/stretchr/testify/assert"
)

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
