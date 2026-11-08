package chat

import (
	"testing"

	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/stretchr/testify/assert"
)

func TestDefaultStatusFormatter(t *testing.T) {
	t.Run("full info", func(t *testing.T) {
		// given
		info := StatusInfo{
			Model:          "gpt-4",
			Provider:       "openai",
			Effort:         llm.EffortMedium,
			ContextPercent: 12.5,
			Tokens:         8400,
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
		assert.Equal(t, "— · ctx: 0% · tokens: 0", result)
	})

	t.Run("effort off omitted", func(t *testing.T) {
		// given
		info := StatusInfo{
			Model:          "claude-3",
			Effort:         llm.EffortOff,
			ContextPercent: 50,
			Tokens:         500,
		}

		// when
		result := defaultStatusFormatter(info)

		// then
		assert.NotContains(t, result, "off")
	})

	t.Run("unknown provider", func(t *testing.T) {
		// given
		info := StatusInfo{
			Model:  "my-model",
			Effort: llm.EffortLow,
		}

		// when
		result := defaultStatusFormatter(info)

		// then
		assert.NotContains(t, result, "()")
	})
}

func TestStatusUsesModelInfo(t *testing.T) {
	// given
	model := &mockedModelBackend{
		info: llm.ModelInfo{
			Name:        "m1",
			Provider:    llm.ProviderOllama,
			ContextSize: 1000,
		},
		effort: llm.EffortLow,
	}
	c, _ := newTestChatWithModel(t, &mockedAgentBackend{}, model)
	agentFeedback{c}.TokensUsed(250, 0)

	// when
	result := c.Status()

	// then
	assert.Equal(t, "m1", result.Model)
	assert.Equal(t, llm.ProviderOllama, result.Provider)
	assert.Equal(t, llm.EffortLow, result.Effort)
	assert.Equal(t, 250, result.Tokens)
	assert.InDelta(t, 25.0, result.ContextPercent, 0.001)
}

func TestStatusShowsZeroContextPercentWithoutAContextSize(t *testing.T) {
	// given
	model := &mockedModelBackend{info: llm.ModelInfo{Name: "m1"}}
	c, _ := newTestChatWithModel(t, &mockedAgentBackend{}, model)
	agentFeedback{c}.TokensUsed(250, 0)

	// when
	result := c.Status()

	// then
	assert.Equal(t, "m1", result.Model)
	assert.Zero(t, result.ContextPercent)
}

func TestStatusShowsTheModelChosenWithTheModelCommand(t *testing.T) {
	// given
	model := &mockedModelBackend{
		info:   llm.ModelInfo{Name: "m1"},
		models: []string{"m1", "other"},
	}
	c, _ := newTestChatWithModel(t, &mockedAgentBackend{}, model, WithModelCommand())

	// when
	c.Submit("/model other")
	waitIdle(t, c)

	// then
	assert.Equal(t, "other", c.Status().Model)
}

func TestStatusShowsTheEffortChosenWithTheEffortCommand(t *testing.T) {
	// given
	model := &mockedModelBackend{effort: llm.EffortOff}
	c, _ := newTestChatWithModel(t, &mockedAgentBackend{}, model, WithEffortCommand())

	// when
	c.Submit("/effort low")
	waitIdle(t, c)

	// then
	assert.Equal(t, llm.EffortLow, c.Status().Effort)
}

func TestStatusStripsControlSequencesFromTheModelName(t *testing.T) {
	// given
	model := &mockedModelBackend{info: llm.ModelInfo{Name: "m\x1b]0;pwned\x07one"}}
	c, _ := newTestChatWithModel(t, &mockedAgentBackend{}, model)

	// when
	result := c.Status()

	// then
	assert.Equal(t, "mone", result.Model)
}
