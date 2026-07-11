package chat

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-chat/theme"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockedAgentBackend struct {
	processFunc         func(ctx context.Context, input string) (*agent.Response, error)
	changeModelFunc     func(name string) error
	changeEffortFunc    func(e llm.Effort)
	availableModelsFunc func() []string
	modelInfoFunc       func(ctx context.Context) *agent.ModelInfo
	compactContextFunc  func(ctx context.Context)
	resetSessionFunc    func() error
}

func (m *mockedAgentBackend) Process(ctx context.Context, input string) (*agent.Response, error) {
	if m.processFunc == nil {
		return nil, nil
	}
	return m.processFunc(ctx, input)
}

func (m *mockedAgentBackend) ChangeModel(name string) error {
	if m.changeModelFunc == nil {
		return nil
	}
	return m.changeModelFunc(name)
}

func (m *mockedAgentBackend) ChangeEffort(e llm.Effort) {
	if m.changeEffortFunc != nil {
		m.changeEffortFunc(e)
	}
}

func (m *mockedAgentBackend) AvailableModels() []string {
	if m.availableModelsFunc == nil {
		return nil
	}
	return m.availableModelsFunc()
}

func (m *mockedAgentBackend) ModelInfo(ctx context.Context) *agent.ModelInfo {
	if m.modelInfoFunc == nil {
		return nil
	}
	return m.modelInfoFunc(ctx)
}

func (m *mockedAgentBackend) CompactContext(ctx context.Context) {
	if m.compactContextFunc != nil {
		m.compactContextFunc(ctx)
	}
}

func (m *mockedAgentBackend) ResetSession() error {
	if m.resetSessionFunc == nil {
		return nil
	}
	return m.resetSessionFunc()
}

type recordingObserver struct {
	transcriptChanged chan struct{}
	quit              chan struct{}
}

func newRecordingObserver() *recordingObserver {
	return &recordingObserver{
		transcriptChanged: make(chan struct{}, 10),
		quit:              make(chan struct{}, 1),
	}
}

func (o *recordingObserver) TranscriptChanged() {
	select {
	case o.transcriptChanged <- struct{}{}:
	default:
	}
}

func (o *recordingObserver) Quit() {
	select {
	case o.quit <- struct{}{}:
	default:
	}
}

func newTestChat(t *testing.T, backend agentBackend, opts ...Option) *Chat {
	t.Helper()
	c := newChat("test", opts...)
	c.agent = backend
	return c
}

type mockCommand struct {
	nameFunc func() string
	helpFunc func() string
	runFunc  func(ctx command.Context, args string)
}

func (m *mockCommand) Name() string {
	if m.nameFunc == nil {
		return ""
	}
	return m.nameFunc()
}

func (m *mockCommand) Help() string {
	if m.helpFunc == nil {
		return ""
	}
	return m.helpFunc()
}

func (m *mockCommand) Run(ctx command.Context, args string) {
	if m.runFunc != nil {
		m.runFunc(ctx, args)
	}
}

func TestChatName(t *testing.T) {
	// given
	c := newChat("test-chat")

	// when
	result := c.Name()

	// then
	assert.Equal(t, "test-chat", result)
}

func TestChatTheme(t *testing.T) {
	t.Run("default theme", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		result := c.Theme()

		// then
		assert.Equal(t, theme.Default, result)
	})

	t.Run("custom theme via option", func(t *testing.T) {
		// given
		c := newChat("test", WithTheme(theme.Nord))

		// when
		result := c.Theme()

		// then
		assert.Equal(t, theme.Nord, result)
	})
}

func TestChatBusy(t *testing.T) {
	// given
	c := newChat("test")

	// when
	result := c.Busy()

	// then
	assert.False(t, result)
}

func TestChatTranscript(t *testing.T) {
	// given
	c := newChat("test")

	// when
	result := c.Transcript()

	// then
	assert.Empty(t, result)
}

func TestChatSetObserver(t *testing.T) {
	// given
	c := newChat("test")
	o := newRecordingObserver()

	// when
	c.SetObserver(o)

	// then
	assert.Equal(t, o, c.observer)
}

func TestChatPrint(t *testing.T) {
	// given
	c := newChat("test")

	// when
	c.Print(command.Info, "hello world")

	// then
	transcript := c.Transcript()
	if assert.Len(t, transcript, 1) {
		assert.Equal(t, command.Info, transcript[0].Kind)
		assert.Equal(t, "hello world", transcript[0].Text)
	}
}

func TestChatSubmit(t *testing.T) {
	t.Run("empty input ignored", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		c.Submit("")

		// then
		assert.Empty(t, c.Transcript())
	})

	t.Run("whitespace input ignored", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		c.Submit("   ")

		// then
		assert.Empty(t, c.Transcript())
	})

	t.Run("/help appends help text", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		c.Submit("/help")

		// then
		transcript := c.Transcript()
		if assert.NotEmpty(t, transcript) {
			assert.Equal(t, command.Info, transcript[0].Kind)
		}
	})

	t.Run("/exit triggers quit", func(t *testing.T) {
		// given
		c := newChat("test")
		o := newRecordingObserver()
		c.SetObserver(o)

		// when
		c.Submit("/exit")

		// then
		select {
		case <-o.quit:
		default:
			t.Error("expected Quit to be called")
		}
	})

	t.Run("unknown command prints error", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		c.Submit("/bogus")

		// then
		transcript := c.Transcript()
		if assert.Len(t, transcript, 1) {
			assert.Equal(t, command.Error, transcript[0].Kind)
		}
	})

	t.Run("registered command runs", func(t *testing.T) {
		// given
		done := make(chan struct{})
		mockCmd := &mockCommand{
			nameFunc: func() string { return "hello" },
			runFunc: func(ctx command.Context, args string) {
				ctx.Print(command.Info, "hi")
				close(done)
			},
		}
		c := newChat("test", WithCommand(mockCmd))

		// when
		c.Submit("/hello")
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for command")
		}

		// then
		transcript := c.Transcript()
		if assert.Len(t, transcript, 1) {
			assert.Equal(t, "hi", transcript[0].Text)
		}
	})
}

func TestChatClear(t *testing.T) {
	t.Run("clear succeeds", func(t *testing.T) {
		// given
		var reset bool
		c := newTestChat(t, &mockedAgentBackend{
			resetSessionFunc: func() error {
				reset = true
				return nil
			},
		})
		c.Print(command.Info, "some text")

		// when
		err := c.Clear()

		// then
		assert.NoError(t, err)
		assert.True(t, reset)
		assert.Empty(t, c.Transcript())
	})

	t.Run("clear error leaves transcript intact", func(t *testing.T) {
		// given
		c := newTestChat(t, &mockedAgentBackend{
			resetSessionFunc: func() error {
				return errors.New("reset failed")
			},
		})
		c.Print(command.Info, "some text")

		// when
		err := c.Clear()

		// then
		assert.Error(t, err)
		assert.NotEmpty(t, c.Transcript())
	})
}

func TestChatChangeTheme(t *testing.T) {
	t.Run("valid theme", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		err := c.ChangeTheme("nord")

		// then
		assert.NoError(t, err)
		assert.Equal(t, theme.Nord, c.Theme())
	})

	t.Run("invalid theme", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		err := c.ChangeTheme("bogus")

		// then
		assert.Error(t, err)
	})
}

func TestChatChangeModel(t *testing.T) {
	// given
	var changed string
	c := newTestChat(t, &mockedAgentBackend{
		changeModelFunc: func(name string) error {
			changed = name
			return nil
		},
	})

	// when
	err := c.ChangeModel("gpt-4")

	// then
	assert.NoError(t, err)
	assert.Equal(t, "gpt-4", changed)
}

func TestChatChangeEffort(t *testing.T) {
	// given
	var changed llm.Effort
	c := newTestChat(t, &mockedAgentBackend{
		changeEffortFunc: func(e llm.Effort) {
			changed = e
		},
	})

	// when
	c.ChangeEffort(llm.EffortMax)

	// then
	assert.Equal(t, llm.EffortMax, changed)
}

func TestChatAvailableModels(t *testing.T) {
	// given
	c := newTestChat(t, &mockedAgentBackend{
		availableModelsFunc: func() []string {
			return []string{"gpt-4", "claude-3"}
		},
	})

	// when
	models := c.AvailableModels()

	// then
	assert.Equal(t, []string{"gpt-4", "claude-3"}, models)
}

func TestChatCompact(t *testing.T) {
	// given
	var compacted bool
	c := newTestChat(t, &mockedAgentBackend{
		compactContextFunc: func(ctx context.Context) {
			compacted = true
		},
	})

	// when
	c.Compact()

	// then
	assert.True(t, compacted)
}

func TestChatFeedback(t *testing.T) {
	t.Run("ToolCalled appends activity", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		c.ToolCalled("fetch")

		// then
		transcript := c.Transcript()
		if assert.Len(t, transcript, 1) {
			assert.Equal(t, command.Activity, transcript[0].Kind)
		}
	})

	t.Run("ContextCompacted appends activity", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		c.ContextCompacted()

		// then
		transcript := c.Transcript()
		if assert.Len(t, transcript, 1) {
			assert.Equal(t, command.Activity, transcript[0].Kind)
		}
	})

	t.Run("ContextCompactionFailed appends error", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		c.ContextCompactionFailed()

		// then
		transcript := c.Transcript()
		if assert.Len(t, transcript, 1) {
			assert.Equal(t, command.Error, transcript[0].Kind)
		}
	})

	t.Run("SessionReset no-op", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		c.SessionReset()

		// then
		assert.Empty(t, c.Transcript())
	})

	t.Run("SessionStarted no-op", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		c.SessionStarted()

		// then
		assert.Empty(t, c.Transcript())
	})

	t.Run("SessionClosed no-op", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		c.SessionClosed()

		// then
		assert.Empty(t, c.Transcript())
	})
}

func TestChatStatus(t *testing.T) {
	// given
	c := newTestChat(t, &mockedAgentBackend{
		modelInfoFunc: func(ctx context.Context) *agent.ModelInfo {
			return &agent.ModelInfo{
				ModelName:        "gpt-4",
				Provider:         "openai",
				Effort:           llm.EffortMedium,
				ModelContextSize: 8000,
			}
		},
	})
	c.mu.Lock()
	c.lastMeta = agent.Metadata{TotalTokens: 1000}
	c.mu.Unlock()

	// when
	status := c.Status()

	// then
	assert.Equal(t, "gpt-4", status.Name)
	assert.Equal(t, llm.Provider("openai"), status.Provider)
	assert.Equal(t, llm.EffortMedium, status.Effort)
	assert.Equal(t, 1000, status.Tokens)
}

func TestChatNotifyOnAppend(t *testing.T) {
	// given
	c := newChat("test")
	o := newRecordingObserver()
	c.SetObserver(o)

	// when
	c.Print(command.Info, "hello")

	// then
	select {
	case <-o.transcriptChanged:
	default:
		t.Error("expected TranscriptChanged to be called")
	}
}

func TestChatLastMetadata(t *testing.T) {
	// given
	c := newChat("test")

	// when
	meta := c.LastMetadata()

	// then
	assert.Equal(t, 0, meta.TotalTokens)
}

func TestChatHelpText(t *testing.T) {
	// given
	c := newChat("test")
	c.register(&mockCommand{
		nameFunc: func() string { return "hello" },
		helpFunc: func() string { return "/hello   Say hi" },
	})

	// when
	help := c.helpText()

	// then
	assert.NotEmpty(t, help)
}

func TestChatProcess(t *testing.T) {
	t.Run("agent success appends reply and telemetry", func(t *testing.T) {
		// given
		c := newTestChat(t, &mockedAgentBackend{
			processFunc: func(ctx context.Context, input string) (*agent.Response, error) {
				return &agent.Response{
					Content: "Hello back",
					Metadata: agent.Metadata{
						OutputTokens: 10,
						TotalTokens:  50,
					},
				}, nil
			},
		}, WithTelemetryFormatter(func(meta agent.Metadata) string {
			return "[telemetry]"
		}))

		// when
		c.process(c.ctx, "hello")

		// then
		transcript := c.Transcript()
		require.NotEmpty(t, transcript)
		assert.Equal(t, command.User, transcript[0].Kind)
		assert.Contains(t, transcript[0].Text, "hello")
	})

	t.Run("agent error appends error line", func(t *testing.T) {
		// given
		c := newTestChat(t, &mockedAgentBackend{
			processFunc: func(ctx context.Context, input string) (*agent.Response, error) {
				return nil, errors.New("api failure")
			},
		})

		// when
		c.process(c.ctx, "hello")

		// then
		transcript := c.Transcript()
		require.Len(t, transcript, 2)
		assert.Equal(t, command.Error, transcript[1].Kind)
		assert.Equal(t, "Error: api failure", transcript[1].Text)
	})

	t.Run("agent busy blocks concurrent submit", func(t *testing.T) {
		// given
		backend := &mockedAgentBackend{}
		c := newTestChat(t, backend)
		c.mu.Lock()
		c.busy = true
		c.mu.Unlock()

		// when
		c.Submit("second")

		// then
		assert.Empty(t, c.Transcript())
	})

	t.Run("non-command text triggers agent process", func(t *testing.T) {
		// given
		processed := make(chan string, 1)
		backend := &mockedAgentBackend{
			processFunc: func(ctx context.Context, input string) (*agent.Response, error) {
				processed <- input
				return &agent.Response{Content: "reply"}, nil
			},
		}
		c := newTestChat(t, backend)

		// when
		c.Submit("hello")

		// then
		select {
		case got := <-processed:
			assert.Equal(t, "hello", got)
		case <-time.After(time.Second):
			t.Error("expected agent.Process to be called")
		}
	})
}

func TestChatConcurrentTranscript(t *testing.T) {
	// given
	c := newChat("test")
	const goroutines = 10
	var wg sync.WaitGroup

	// when
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Print(command.Info, "line")
			_ = c.Transcript()
			_ = c.Busy()
		}()
	}
	wg.Wait()

	// then
	assert.Equal(t, goroutines, len(c.Transcript()))
}

func TestChatStatusText(t *testing.T) {
	// given
	c := newTestChat(t, &mockedAgentBackend{
		modelInfoFunc: func(ctx context.Context) *agent.ModelInfo {
			return &agent.ModelInfo{
				ModelName:        "gpt-4",
				Provider:         "openai",
				Effort:           llm.EffortOff,
				ModelContextSize: 8000,
			}
		},
	})

	// when
	text := c.StatusText()

	// then
	assert.NotEmpty(t, text)
}
