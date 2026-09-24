package chat

import (
	"context"
	"testing"
	"time"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	c.TokensUsed(250)

	// when
	result := waitStatus(t, c, func(s StatusInfo) bool { return s.Name != "" })

	// then
	assert.Equal(t, "m1", result.Name)
	assert.Equal(t, llm.ProviderOllama, result.Provider)
	assert.Equal(t, llm.EffortLow, result.Effort)
	assert.Equal(t, 250, result.Tokens)
	assert.InDelta(t, 25.0, result.CtxPct, 0.001)
}

func TestStatusReturnsWithoutWaitingForTheLookup(t *testing.T) {
	// given
	release := make(chan struct{})
	backend := &mockedAgentBackend{
		modelInfoFunc: func(context.Context) *agent.ModelInfo {
			<-release
			return &agent.ModelInfo{ModelName: "m1"}
		},
	}
	c, _ := newTestChat(t, backend)
	defer close(release)

	// when
	result := c.Status()

	// then
	assert.Empty(t, result.Name)
}

func TestStatusNotifiesTheObserverWhenTheLookupLands(t *testing.T) {
	// given
	backend := &mockedAgentBackend{
		modelInfoFunc: func(context.Context) *agent.ModelInfo {
			return &agent.ModelInfo{ModelName: "m1"}
		},
	}
	c, obs := newTestChat(t, backend)

	// when
	waitStatus(t, c, func(s StatusInfo) bool { return s.Name == "m1" })

	// then
	obs.mu.Lock()
	defer obs.mu.Unlock()
	assert.Positive(t, obs.changes)
}

func TestStatusLooksTheModelUpOnce(t *testing.T) {
	// given
	backend := &mockedAgentBackend{
		modelInfoFunc: func(context.Context) *agent.ModelInfo {
			return &agent.ModelInfo{ModelName: "m1", ModelContextSize: 1000}
		},
	}
	c, _ := newTestChat(t, backend)
	waitStatus(t, c, func(s StatusInfo) bool { return s.Name == "m1" })

	// when
	for range 10 {
		c.Status()
	}

	// then
	assert.Equal(t, 1, backend.infoHits())
}

func TestStatusLooksTheModelUpAgainAfterASwitch(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Chat)
	}{
		{name: "changing model", change: func(c *Chat) { _ = c.ChangeModel("m2") }},
		{name: "changing effort", change: func(c *Chat) { _ = c.ChangeEffort(llm.EffortMax) }},
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
			waitStatus(t, c, func(s StatusInfo) bool { return s.Name == "m1" })

			// when
			tc.change(c)
			c.Status()

			// then
			require.Eventually(t, func() bool { return backend.infoHits() == 2 }, 2*time.Second, time.Millisecond)
		})
	}
}

func TestStatusRetriesAFailedLookupOnlyAfterATurn(t *testing.T) {
	// given
	backend := &mockedAgentBackend{
		modelInfoFunc: func(context.Context) *agent.ModelInfo { return nil },
	}
	c, _ := newTestChat(t, backend)
	c.Status()
	require.Eventually(t, func() bool { return backend.infoHits() == 1 }, 2*time.Second, time.Millisecond)
	for range 10 {
		c.Status()
	}
	require.Equal(t, 1, backend.infoHits())

	// when
	c.Submit("hi")
	waitIdle(t, c)
	c.Status()

	// then
	require.Eventually(t, func() bool { return backend.infoHits() == 2 }, 2*time.Second, time.Millisecond)
}

func TestStatusDoesNotLoopWhenTheModelIsUnavailable(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	backend.modelInfoFunc = func(context.Context) *agent.ModelInfo {
		c.ModelInfoUnavailable()
		return nil
	}
	c.SetObserver(&renderingObserver{core: c})

	// when
	c.Status()

	// then
	require.Eventually(t, func() bool { return backend.infoHits() >= 1 }, 2*time.Second, time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, backend.infoHits())
	assert.Len(t, c.Transcript(), 1)
}

func TestModelInfoUnavailableIsReportedOncePerModel(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	c.ModelInfoUnavailable()

	// when
	c.ModelInfoUnavailable()
	_ = c.ChangeModel("m2")
	c.ModelInfoUnavailable()

	// then
	assert.Len(t, c.Transcript(), 2)
}

func TestStatusDoesNotQueryTheAgentWhileATurnRuns(t *testing.T) {
	// given
	started := make(chan bool, 1)
	release := make(chan struct{})
	backend := &mockedAgentBackend{
		processFunc: func(context.Context, string) (*agent.Response, error) {
			started <- true
			<-release
			return &agent.Response{Content: "ok"}, nil
		},
		modelInfoFunc: func(context.Context) *agent.ModelInfo {
			return &agent.ModelInfo{ModelName: "m1"}
		},
	}
	c, _ := newTestChat(t, backend)
	c.Submit("hi")
	<-started

	// when
	c.Status()
	time.Sleep(20 * time.Millisecond)
	hitsDuringTurn := backend.infoHits()
	close(release)

	// then
	assert.Zero(t, hitsDuringTurn)
	waitStatus(t, c, func(s StatusInfo) bool { return s.Name == "m1" })
}

func TestStatusStripsControlSequencesFromTheModelName(t *testing.T) {
	// given
	backend := &mockedAgentBackend{
		modelInfoFunc: func(context.Context) *agent.ModelInfo {
			return &agent.ModelInfo{ModelName: "m\x1b]0;pwned\x07one"}
		},
	}
	c, _ := newTestChat(t, backend)

	// when
	result := waitStatus(t, c, func(s StatusInfo) bool { return s.Name != "" })

	// then
	assert.Equal(t, "mone", result.Name)
}
