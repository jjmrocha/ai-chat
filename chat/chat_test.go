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
			assert.Zero(t, c.TranscriptLen())
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
	assert.Equal(t, command.User, lines[0].Kind)
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
	assert.Equal(t, command.Error, lines[1].Kind)
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
	assert.Equal(t, command.Error, lines[1].Kind)
	assert.Equal(t, "No response received.", lines[1].Text)
}

func TestUnknownCommandReportsAndStaysUsable(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)

	// when
	c.Submit("/nope arg")
	waitIdle(t, c)

	// then
	lines := c.Transcript()
	require.Len(t, lines, 1)
	assert.Equal(t, command.Error, lines[0].Kind)
	assert.Equal(t, "Error: unknown command /nope", lines[0].Text)
	assert.False(t, c.Busy())
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
	assert.True(t, c.Queued())
	close(release)
	waitIdle(t, c)
	assert.False(t, c.Queued())
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
	assert.NotPanics(t, c.Quit)
}

func TestTranscriptLenAndSince(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	for i := 0; i < 5; i++ {
		c.append(command.Info, "line")
	}

	tests := []struct {
		name        string
		from        int
		expectedLen int
	}{
		{name: "from zero returns everything", from: 0, expectedLen: 5},
		{name: "from the middle returns the tail", from: 3, expectedLen: 2},
		{name: "from the end returns nothing", from: 5, expectedLen: 0},
		{name: "past the end returns nothing", from: 99, expectedLen: 0},
		{name: "negative returns nothing", from: -1, expectedLen: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			result := c.Since(tc.from)

			// then
			assert.Len(t, result, tc.expectedLen)
		})
	}

	assert.Equal(t, 5, c.TranscriptLen())
}

func TestSinceReturnsACopy(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	c.append(command.Info, "original")

	// when
	result := c.Since(0)
	result[0].Text = "mutated"

	// then
	assert.Equal(t, "original", c.Transcript()[0].Text)
}

func TestClearResetsTranscript(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	c.append(command.Info, "line")

	// when
	err := c.Clear()

	// then
	assert.NoError(t, err)
	assert.Zero(t, c.TranscriptLen())
}

func TestClearPropagatesResetFailure(t *testing.T) {
	// given
	backend := &mockedAgentBackend{
		resetSessionFunc: func() error { return errors.New("locked") },
	}
	c, _ := newTestChat(t, backend)
	c.append(command.Info, "line")

	// when
	err := c.Clear()

	// then
	assert.EqualError(t, err, "locked")
	assert.Equal(t, 1, c.TranscriptLen())
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
	assert.False(t, c.Queued())
	for _, ln := range c.Transcript() {
		assert.NotEqual(t, command.Error, ln.Kind)
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
