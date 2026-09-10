package chat

import (
	"context"
	"errors"
	"strings"
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
	changeEffortFunc    func(e llm.Effort) error
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

func (m *mockedAgentBackend) ChangeEffort(e llm.Effort) error {
	if m.changeEffortFunc == nil {
		return nil
	}
	return m.changeEffortFunc(e)
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

// mockArgCommand is a mockCommand that also advertises an argument spec.
type mockArgCommand struct {
	mockCommand
	args string
}

func (m *mockArgCommand) Args() string { return m.args }

func TestHelpTextAlignment(t *testing.T) {
	t.Run("descriptions line up past the widest usage", func(t *testing.T) {
		// given
		wide := &mockArgCommand{
			mockCommand: mockCommand{
				nameFunc: func() string { return "wide" },
				helpFunc: func() string { return "Takes a long spec" },
			},
			args: "[on|off] [name]",
		}
		narrow := &mockCommand{
			nameFunc: func() string { return "narrow" },
			helpFunc: func() string { return "Takes nothing" },
		}
		c := newChat("test", WithCommand(wide), WithCommand(narrow))

		// when
		result := c.helpText()

		// then
		assert.Equal(t, strings.Join([]string{
			"Commands:",
			"  /narrow               Takes nothing",
			"  /wide [on|off] [name] Takes a long spec",
			"  /help                 Show this message",
			"  /exit                 Quit",
		}, "\n"), result)
	})

	t.Run("a command without args renders just its name", func(t *testing.T) {
		// given
		only := &mockCommand{
			nameFunc: func() string { return "solo" },
			helpFunc: func() string { return "Does a thing" },
		}
		c := newChat("test", WithCommand(only))

		// when
		result := c.helpText()

		// then
		assert.Contains(t, result, "  /solo Does a thing")
	})

	t.Run("an empty args spec adds no trailing space", func(t *testing.T) {
		// given
		blank := &mockArgCommand{
			mockCommand: mockCommand{
				nameFunc: func() string { return "blank" },
				helpFunc: func() string { return "Does a thing" },
			},
			args: "",
		}
		c := newChat("test", WithCommand(blank))

		// when
		result := c.helpText()

		// then
		assert.Contains(t, result, "  /blank Does a thing")
	})
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
	t.Run("forwards the level and reports success", func(t *testing.T) {
		// given
		var changed llm.Effort
		c := newTestChat(t, &mockedAgentBackend{
			changeEffortFunc: func(e llm.Effort) error {
				changed = e
				return nil
			},
		})

		// when
		err := c.ChangeEffort(llm.EffortMax)

		// then
		assert.NoError(t, err)
		assert.Equal(t, llm.EffortMax, changed)
	})

	t.Run("propagates the agent error", func(t *testing.T) {
		// given
		c := newTestChat(t, &mockedAgentBackend{
			changeEffortFunc: func(llm.Effort) error {
				return llm.ErrInvalidEffort
			},
		})

		// when
		err := c.ChangeEffort("extreme")

		// then
		assert.ErrorIs(t, err, llm.ErrInvalidEffort)
	})
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
	t.Run("ToolCalled appends activity with the call rendered", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		c.ToolCalled("fetch", map[string]any{"url": "http://a", "depth": float64(2)})

		// then
		transcript := c.Transcript()
		if assert.Len(t, transcript, 1) {
			assert.Equal(t, command.Activity, transcript[0].Kind)
			assert.Equal(t, `● fetch(depth=2, url="http://a")`, transcript[0].Text)
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

	t.Run("ModelInfoUnavailable appends error", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		c.ModelInfoUnavailable()

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

func TestChatSetContext(t *testing.T) {
	// given
	c := newChat("test")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// when
	c.SetContext(ctx)

	// then - the cancelled context should propagate to agent calls
	// (no crash on SetContext itself)
	assert.Equal(t, ctx, c.baseCtx)
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
		c.process(c.baseCtx, "hello")

		// then
		transcript := c.Transcript()
		require.NotEmpty(t, transcript)
		assert.Equal(t, command.User, transcript[0].Kind)
		assert.Equal(t, "❯ hello", transcript[0].Text, "the echo keeps its prompt marker")
	})

	t.Run("agent error appends error line", func(t *testing.T) {
		// given
		c := newTestChat(t, &mockedAgentBackend{
			processFunc: func(ctx context.Context, input string) (*agent.Response, error) {
				return nil, errors.New("api failure")
			},
		})

		// when
		c.process(c.baseCtx, "hello")

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

	t.Run("nil response appends error", func(t *testing.T) {
		// given
		c := newTestChat(t, &mockedAgentBackend{
			processFunc: func(ctx context.Context, input string) (*agent.Response, error) {
				return nil, nil
			},
		})

		// when
		c.process(c.baseCtx, "hello")

		// then
		transcript := c.Transcript()
		require.Len(t, transcript, 2)
		assert.Equal(t, command.Error, transcript[1].Kind)
		assert.Equal(t, "No response received.", transcript[1].Text)
	})
}

func TestChatConcurrentTranscript(t *testing.T) {
	// given
	c := newChat("test")
	const goroutines = 10
	var wg sync.WaitGroup

	// when
	for range goroutines {
		wg.Go(func() {
			c.Print(command.Info, "line")
			_ = c.Transcript()
			_ = c.Busy()
		})
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

func TestFormatToolCall(t *testing.T) {
	long := strings.Repeat("x", maxToolArgLen+1)

	testCases := []struct {
		name     string
		tool     string
		args     map[string]any
		expected string
	}{
		{
			name:     "no arguments",
			tool:     "repo_info",
			args:     nil,
			expected: "● repo_info()",
		},
		{
			name:     "empty arguments",
			tool:     "repo_info",
			args:     map[string]any{},
			expected: "● repo_info()",
		},
		{
			name:     "quotes strings and prints numbers bare",
			tool:     "lookup",
			args:     map[string]any{"name": "", "age": float64(1)},
			expected: `● lookup(age=1, name="")`,
		},
		{
			name:     "sorts arguments by name",
			tool:     "edit",
			args:     map[string]any{"c": true, "a": float64(1), "b": "x"},
			expected: `● edit(a=1, b="x", c=true)`,
		},
		{
			name:     "prints a fractional number as written",
			tool:     "sample",
			args:     map[string]any{"ratio": 1.5},
			expected: "● sample(ratio=1.5)",
		},
		{
			name:     "elides a long string",
			tool:     "file_write",
			args:     map[string]any{"path": "notes.md", "content": long},
			expected: `● file_write(content: ..., path="notes.md")`,
		},
		{
			name:     "keeps a string at the limit",
			tool:     "file_write",
			args:     map[string]any{"content": strings.Repeat("x", maxToolArgLen)},
			expected: `● file_write(content="` + strings.Repeat("x", maxToolArgLen) + `")`,
		},
		{
			name:     "elides a nested object",
			tool:     "edit",
			args:     map[string]any{"path": "a.md", "spec": map[string]any{"k": "v"}},
			expected: `● edit(path="a.md", spec: ...)`,
		},
		{
			name:     "elides a list",
			tool:     "batch",
			args:     map[string]any{"items": []any{1, 2}, "dry": false},
			expected: "● batch(dry=false, items: ...)",
		},
		{
			name:     "prints a null argument",
			tool:     "search",
			args:     map[string]any{"filter": nil},
			expected: "● search(filter=null)",
		},
		{
			name:     "escapes a string with quotes and newlines",
			tool:     "say",
			args:     map[string]any{"text": "a\"b\nc"},
			expected: `● say(text="a\"b\nc")`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			result := formatToolCall(tc.tool, tc.args)
			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestChatQueue(t *testing.T) {
	t.Run("nothing is queued on an idle chat", func(t *testing.T) {
		// given
		c := newChat("test")

		// when
		result := c.Queued()

		// then
		assert.False(t, result)
	})

	t.Run("input submitted during a turn is queued, not dropped", func(t *testing.T) {
		// given
		started, release := make(chan struct{}, 1), make(chan struct{})
		c := newTestChat(t, &mockedAgentBackend{
			processFunc: func(ctx context.Context, input string) (*agent.Response, error) {
				select {
				case started <- struct{}{}:
				default:
				}
				<-release
				return &agent.Response{Content: "reply"}, nil
			},
		})
		c.Submit("first")
		<-started
		defer close(release)

		// when
		c.Submit("second")

		// then
		assert.True(t, c.Queued())
	})

	t.Run("the queue drains in submission order once the turn ends", func(t *testing.T) {
		// given
		var mu sync.Mutex
		var seen []string
		started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
		c := newTestChat(t, &mockedAgentBackend{
			processFunc: func(ctx context.Context, input string) (*agent.Response, error) {
				mu.Lock()
				seen = append(seen, input)
				count := len(seen)
				mu.Unlock()
				switch count {
				case 1:
					close(started)
					<-release
				case 3:
					close(done)
				}
				return &agent.Response{Content: "reply"}, nil
			},
		})
		c.Submit("first")
		<-started

		// when
		c.Submit("second")
		c.Submit("third")
		close(release)

		// then
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for the queue to drain")
		}
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, []string{"first", "second", "third"}, seen)
	})

	t.Run("a command queued during a turn runs after it", func(t *testing.T) {
		// given
		started, release, ran := make(chan struct{}), make(chan struct{}), make(chan struct{})
		mockCmd := &mockCommand{
			nameFunc: func() string { return "hello" },
			runFunc:  func(ctx command.Context, args string) { close(ran) },
		}
		c := newTestChat(t, &mockedAgentBackend{
			processFunc: func(ctx context.Context, input string) (*agent.Response, error) {
				close(started)
				<-release
				return &agent.Response{Content: "reply"}, nil
			},
		}, WithCommand(mockCmd))
		c.Submit("first")
		<-started

		// when
		c.Submit("/hello")
		close(release)

		// then
		select {
		case <-ran:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for the queued command")
		}
	})
}

func TestChatQueueOrdering(t *testing.T) {
	// given
	started, release := make(chan struct{}), make(chan struct{})
	var c *Chat
	c = newTestChat(t, &mockedAgentBackend{
		processFunc: func(ctx context.Context, input string) (*agent.Response, error) {
			c.ToolCalled("file_workdir", nil)
			close(started)
			<-release
			return &agent.Response{Content: "the folder is ai-chat"}, nil
		},
	})
	o := newRecordingObserver()
	c.SetObserver(o)
	c.Submit("what is the name of this folder?")
	<-started

	// when
	c.Submit("/help")
	close(release)

	// then
	require.Eventually(t, func() bool {
		return !c.Busy() && !c.Queued()
	}, time.Second, 5*time.Millisecond)

	kinds := make([]command.Kind, 0)
	for _, ln := range c.Transcript() {
		kinds = append(kinds, ln.Kind)
	}
	assert.Equal(t, []command.Kind{
		command.User, command.Activity, command.Reply, command.Info,
	}, kinds, "a queued command reaches the transcript after the turn it waited on")
}
