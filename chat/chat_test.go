package chat

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitIgnoresBlankInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "spaces", input: "   "},
		{name: "tabs and newlines", input: "\t\n "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			backend := &mockedAgentBackend{}
			c, _ := newTestChat(t, backend)

			// when
			c.Submit(tc.input)

			// then
			assert.Empty(t, backend.inputs())
			assert.Zero(t, len(c.Transcript()))
			assert.False(t, c.Busy())
		})
	}
}

func TestSubmitTrimsInput(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)

	// when
	c.Submit("  hello  ")
	waitIdle(t, c)

	// then
	assert.Equal(t, []string{"hello"}, backend.inputs())
}

func TestTurnAppendsUserLineWithoutGlyph(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)

	// when
	c.Submit("hi")
	waitIdle(t, c)

	// then
	lines := c.Transcript()
	require.NotEmpty(t, lines)
	assert.Equal(t, User, lines[0].Kind)
	assert.Equal(t, "hi", lines[0].Text)
}

func TestTurnReportsAgentError(t *testing.T) {
	// given
	backend := &mockedAgentBackend{
		processFunc: func(context.Context, string) (*agent.Response, error) {
			return nil, errors.New("upstream down")
		},
	}
	c, _ := newTestChat(t, backend)

	// when
	c.Submit("hi")
	waitIdle(t, c)

	// then
	lines := c.Transcript()
	require.Len(t, lines, 2)
	assert.Equal(t, Error, lines[1].Kind)
	assert.Equal(t, "Error: upstream down", lines[1].Text)
}

func TestTurnReportsMissingResponse(t *testing.T) {
	// given
	backend := &mockedAgentBackend{
		processFunc: func(context.Context, string) (*agent.Response, error) {
			return nil, nil
		},
	}
	c, _ := newTestChat(t, backend)

	// when
	c.Submit("hi")
	waitIdle(t, c)

	// then
	lines := c.Transcript()
	require.Len(t, lines, 2)
	assert.Equal(t, Error, lines[1].Kind)
	assert.Equal(t, "No response received.", lines[1].Text)
}

func TestUnknownSlashInputIsSentToTheAgentAsTyped(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "path", input: "/Users/me/main.go why does this panic?"},
		{name: "unregistered word", input: "/nope arg"},
		{name: "bare slash", input: "/"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			backend := &mockedAgentBackend{}
			c, _ := newTestChat(t, backend)

			// when
			c.Submit(tc.input)
			waitIdle(t, c)

			// then
			assert.Equal(t, []string{tc.input}, backend.inputs())
			assert.Equal(t, []Line{
				{Kind: User, Text: tc.input},
				{Kind: Reply, Text: "reply"},
			}, c.Transcript())
		})
	}
}

func TestCommandNameEndsAtAnyWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "space", input: "/echo hello world"},
		{name: "newline", input: "/echo\nhello world"},
		{name: "tab", input: "/echo\thello world"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			cmd := &argsCommand{name: "echo"}
			c, _ := newTestChat(t, &mockedAgentBackend{}, WithCommand(cmd))

			// when
			c.Submit(tc.input)
			waitIdle(t, c)

			// then
			assert.Equal(t, "hello world", cmd.args)
		})
	}
}

func TestSkillCommandWithArgumentsOnTheNextLineReachesTheAgent(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend, WithSkillCommand("brainstorm", "Explore"))

	// when
	c.Submit("/brainstorm\nsome idea")
	waitIdle(t, c)

	// then
	assert.Equal(t, []string{"/brainstorm\nsome idea"}, backend.inputs())
}

func TestSkillCommandIsRecordedAsAUserTurn(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend, WithSkillCommand("brainstorm", "Explore"))

	// when
	c.Submit("/brainstorm some idea")
	waitIdle(t, c)

	// then
	lines := c.Transcript()
	require.Len(t, lines, 2)
	assert.Equal(t, User, lines[0].Kind)
	assert.Equal(t, "/brainstorm some idea", lines[0].Text)
	assert.Equal(t, Reply, lines[1].Kind)
	assert.Equal(t, "reply", lines[1].Text)
}

func TestProgressIsThinkingDuringASkillCommand(t *testing.T) {
	// given
	started := make(chan bool, 1)
	release := make(chan struct{})
	backend := &mockedAgentBackend{
		processFunc: func(context.Context, string) (*agent.Response, error) {
			started <- true
			<-release
			return &agent.Response{Content: "reply"}, nil
		},
	}
	c, _ := newTestChat(t, backend, WithSkillCommand("brainstorm", "Explore"))

	// when
	c.Submit("/brainstorm")
	<-started

	// then
	assert.Equal(t, Progress{Stage: Thinking}, c.Progress())
	close(release)
	waitIdle(t, c)
}

func TestCommandsSubmittedWhileIdleDoNotRunConcurrently(t *testing.T) {
	// given
	started := make(chan string, 4)
	release := make(chan struct{})
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend, WithCommand(gatedCommand{
		name:    "hold",
		started: started,
		release: release,
	}))

	// when
	c.Submit("/hold")
	c.Submit("/hold")

	// then
	assert.Equal(t, "hold", <-started)
	select {
	case <-started:
		t.Fatal("second command started before the first finished")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	assert.Equal(t, "hold", <-started)
	waitIdle(t, c)
}

func TestCommandsAndTurnsShareOneQueueInOrder(t *testing.T) {
	// given
	var order []string
	var mu sync.Mutex
	backend := &mockedAgentBackend{
		processFunc: func(context.Context, string) (*agent.Response, error) {
			mu.Lock()
			order = append(order, "turn")
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
			return &agent.Response{Content: "ok"}, nil
		},
	}
	c, _ := newTestChat(t, backend, WithCommand(recordingCommand{name: "mark", log: &order, mu: &mu}))

	// when
	c.Submit("first")
	c.Submit("/mark")
	c.Submit("second")
	waitIdle(t, c)

	// then
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"turn", "mark", "turn"}, order)
}

func TestSlashCommandMarksChatBusy(t *testing.T) {
	// given
	release := make(chan struct{})
	observed := make(chan bool, 1)
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend, WithCommand(blockingCommand{
		name:    "hold",
		started: observed,
		release: release,
	}))

	// when
	c.Submit("/hold")
	<-observed

	// then
	assert.True(t, c.Busy())
	close(release)
	waitIdle(t, c)
	assert.False(t, c.Busy())
}

func TestProgressNamesTheRunningCommand(t *testing.T) {
	// given
	release := make(chan struct{})
	observed := make(chan bool, 1)
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend, WithCommand(blockingCommand{
		name:    "hold",
		started: observed,
		release: release,
	}))

	// when
	c.Submit("/hold there")
	<-observed

	// then
	assert.Equal(t, Progress{Stage: RunningCommand, Detail: "/hold there"}, c.Progress())
	close(release)
	waitIdle(t, c)
	assert.Equal(t, Progress{}, c.Progress())
}

func TestProgressReportsTheToolInFlightOverTheRunningCommand(t *testing.T) {
	// given
	release := make(chan struct{})
	observed := make(chan bool, 1)
	c, _ := newTestChat(t, &mockedAgentBackend{}, WithCommand(blockingCommand{
		name:    "hold",
		started: observed,
		release: release,
	}))
	c.Submit("/hold")
	<-observed

	// when
	agentFeedback{c}.ToolCalled("read", nil)

	// then
	assert.Equal(t, Progress{Stage: RunningTool, Detail: "read()"}, c.Progress())
	close(release)
	waitIdle(t, c)
}

func TestProgressIsThinkingDuringATurn(t *testing.T) {
	// given
	started := make(chan bool, 1)
	release := make(chan struct{})
	backend := &mockedAgentBackend{
		processFunc: func(context.Context, string) (*agent.Response, error) {
			started <- true
			<-release
			return &agent.Response{Content: "reply"}, nil
		},
	}
	c, _ := newTestChat(t, backend)

	// when
	c.Submit("hello")
	<-started

	// then
	assert.Equal(t, Progress{Stage: Thinking}, c.Progress())
	close(release)
	waitIdle(t, c)
}

func TestProgressIsIdleBeforeAnyInput(t *testing.T) {
	// given
	c, _ := newTestChat(t, &mockedAgentBackend{})

	// when
	result := c.Progress()

	// then
	assert.Equal(t, Progress{}, result)
}

func TestProgressReportsCancellingOverTheToolInFlight(t *testing.T) {
	// given
	started := make(chan bool, 1)
	release := make(chan struct{})
	var c *Chat
	backend := &mockedAgentBackend{
		processFunc: func(ctx context.Context, _ string) (*agent.Response, error) {
			agentFeedback{c}.ToolCalled("read", nil)
			started <- true
			<-release
			return nil, ctx.Err()
		},
	}
	c, _ = newTestChat(t, backend)
	c.Submit("hello")
	<-started

	// when
	c.Cancel()

	// then
	assert.Equal(t, Progress{Stage: Cancelling}, c.Progress())
	close(release)
	waitIdle(t, c)
}

func TestQueuedInputIsReportedAndDrained(t *testing.T) {
	// given
	release := make(chan struct{})
	started := make(chan bool, 1)
	backend := &mockedAgentBackend{
		processFunc: func(context.Context, string) (*agent.Response, error) {
			select {
			case started <- true:
			default:
			}
			<-release
			return &agent.Response{Content: "ok"}, nil
		},
	}
	c, _ := newTestChat(t, backend)

	// when
	c.Submit("first")
	<-started
	c.Submit("second")

	// then
	assert.True(t, c.Progress().Queued)
	close(release)
	waitIdle(t, c)
	assert.False(t, c.Progress().Queued)
	assert.Equal(t, []string{"first", "second"}, backend.inputs())
}

func TestExitCommandNotifiesObserver(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, obs := newTestChat(t, backend)

	// when
	c.Submit("/exit")
	waitIdle(t, c)

	// then
	assert.Equal(t, 1, obs.quitCount())
}

func TestQuitWithoutObserverDoesNotPanic(t *testing.T) {
	// given
	c := newChat("TEST")
	c.agent = &mockedAgentBackend{}

	// when / then
	assert.NotPanics(t, commandContext{c}.Quit)
}

func TestClearResetsTranscript(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	c.append(Info, "line")

	// when
	err := c.clear()

	// then
	assert.NoError(t, err)
	assert.Zero(t, len(c.Transcript()))
}

func TestClearPropagatesResetFailure(t *testing.T) {
	// given
	backend := &mockedAgentBackend{
		resetSessionFunc: func() error { return errors.New("locked") },
	}
	c, _ := newTestChat(t, backend)
	c.append(Info, "line")

	// when
	err := c.clear()

	// then
	assert.EqualError(t, err, "locked")
	assert.Equal(t, 1, len(c.Transcript()))
}

func TestConcurrentSubmitsAreSerialized(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend, WithDefaultCommands())

	// when
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				c.Submit("/help")
				return
			}
			c.Submit("msg")
		}(i)
	}
	wg.Wait()
	waitIdle(t, c)

	// then
	assert.Len(t, backend.inputs(), 20)
	assert.False(t, c.Progress().Queued)
	for _, ln := range c.Transcript() {
		assert.NotEqual(t, Error, ln.Kind)
	}
}

func TestHelpListsEveryRegisteredCommand(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend, WithDefaultCommands())

	// when
	c.Submit("/help")
	waitIdle(t, c)

	// then
	lines := c.Transcript()
	require.Len(t, lines, 1)
	for _, want := range []string{"/clear", "/compact", "/effort", "/exit", "/help", "/model"} {
		assert.Contains(t, lines[0].Text, want)
	}
}

type recordingCommand struct {
	name string
	log  *[]string
	mu   *sync.Mutex
}

func (r recordingCommand) Name() string { return r.name }
func (r recordingCommand) Help() string { return "record" }
func (r recordingCommand) Run(command.Context, string) {
	r.mu.Lock()
	*r.log = append(*r.log, r.name)
	r.mu.Unlock()
}

type argsCommand struct {
	name string
	args string
}

func (a *argsCommand) Name() string                       { return a.name }
func (a *argsCommand) Help() string                       { return "args" }
func (a *argsCommand) Run(_ command.Context, args string) { a.args = args }

type blockingCommand struct {
	name    string
	started chan bool
	release chan struct{}
}

func (b blockingCommand) Name() string { return b.name }
func (b blockingCommand) Help() string { return "block" }
func (b blockingCommand) Run(ctx command.Context, _ string) {
	b.started <- true
	<-b.release
}

type gatedCommand struct {
	name    string
	started chan string
	release chan struct{}
}

func (g gatedCommand) Name() string { return g.name }
func (g gatedCommand) Help() string { return "gate" }
func (g gatedCommand) Run(command.Context, string) {
	g.started <- g.name
	<-g.release
}

func blockingUntilCancelled(started chan<- bool) func(context.Context, string) (*agent.Response, error) {
	return func(ctx context.Context, _ string) (*agent.Response, error) {
		select {
		case started <- true:
		default:
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
}

func TestCancelStopsTheRunningTurn(t *testing.T) {
	// given
	started := make(chan bool, 1)
	backend := &mockedAgentBackend{processFunc: blockingUntilCancelled(started)}
	c, _ := newTestChat(t, backend)
	c.Submit("hello")
	<-started

	// when
	c.Cancel()

	// then
	waitIdle(t, c)
	lines := c.Transcript()
	require.NotEmpty(t, lines)
	last := lines[len(lines)-1]
	assert.Equal(t, Info, last.Kind)
	assert.Equal(t, "Cancelled.", last.Text)
	assert.NotEqual(t, Cancelling, c.Progress().Stage)
}

func TestCancellingIsReportedUntilTheTurnReturns(t *testing.T) {
	// given
	started := make(chan bool, 1)
	release := make(chan struct{})
	backend := &mockedAgentBackend{
		processFunc: func(ctx context.Context, _ string) (*agent.Response, error) {
			started <- true
			<-release
			return nil, ctx.Err()
		},
	}
	c, _ := newTestChat(t, backend)
	c.Submit("hello")
	<-started

	// when
	c.Cancel()

	// then
	assert.Equal(t, Cancelling, c.Progress().Stage)
	close(release)
	waitIdle(t, c)
	assert.NotEqual(t, Cancelling, c.Progress().Stage)
}

func TestCancelDropsQueuedInput(t *testing.T) {
	// given
	started := make(chan bool, 1)
	backend := &mockedAgentBackend{processFunc: blockingUntilCancelled(started)}
	c, _ := newTestChat(t, backend)
	c.Submit("first")
	<-started
	c.Submit("second")
	c.Submit("third")

	// when
	c.Cancel()

	// then
	waitIdle(t, c)
	assert.False(t, c.Progress().Queued)
	assert.Equal(t, []string{"first"}, backend.inputs())
}

func TestTurnErrorsOtherThanCancelAreReported(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, c *Chat, backend *mockedAgentBackend)
	}{
		{
			name: "base context cancelled",
			prepare: func(t *testing.T, c *Chat, backend *mockedAgentBackend) {
				ctx, cancel := context.WithCancel(context.Background())
				t.Cleanup(cancel)
				c.SetContext(ctx)
				backend.processFunc = func(ctx context.Context, _ string) (*agent.Response, error) {
					cancel()
					<-ctx.Done()
					return nil, ctx.Err()
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			backend := &mockedAgentBackend{}
			c, _ := newTestChat(t, backend)
			tc.prepare(t, c, backend)

			// when
			c.Submit("hello")

			// then
			waitIdle(t, c)
			lines := c.Transcript()
			require.NotEmpty(t, lines)
			last := lines[len(lines)-1]
			assert.Equal(t, Error, last.Kind)
			assert.Contains(t, last.Text, "Error: ")
		})
	}
}

func TestCancelArrivingWithTheReplyKeepsTheReply(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	backend.processFunc = func(context.Context, string) (*agent.Response, error) {
		c.Cancel()
		return &agent.Response{Content: "answer"}, nil
	}

	// when
	c.Submit("hello")

	// then
	waitIdle(t, c)
	var texts []string
	for _, ln := range c.Transcript() {
		texts = append(texts, ln.Text)
	}
	assert.Contains(t, texts, "answer")
	assert.NotContains(t, texts, "Cancelled.")
}

func TestCancelWhileIdleChangesNothing(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)

	// when
	c.Cancel()

	// then
	assert.Empty(t, c.Transcript())
	assert.NotEqual(t, Cancelling, c.Progress().Stage)
	assert.False(t, c.Busy())
}
