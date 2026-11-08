package chat

import (
	"context"
	"testing"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type blockingCmd struct{ started chan bool }

func (blockingCmd) Name() string { return "block" }

func (blockingCmd) Help() string { return "Block until cancelled" }

func (b blockingCmd) Run(ctx command.Context, _ string) {
	b.started <- true
	<-ctx.Context().Done()
	ctx.Error("Error: " + ctx.Context().Err().Error())
}

func TestCancelRightAfterSubmitStopsTheTurn(t *testing.T) {
	// given
	backend := &mockedAgentBackend{
		processFunc: func(ctx context.Context, _ string) (*agent.Response, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return &agent.Response{Content: "answer"}, nil
		},
	}
	c, _ := newTestChat(t, backend)

	// when
	c.Submit("hello")
	c.Cancel()

	// then
	c.Wait()
	lines := transcript(c)
	require.NotEmpty(t, lines)
	assert.Equal(t, "Cancelled.", lines[len(lines)-1].Text)
}

func TestCancelStopsTheRunningCommand(t *testing.T) {
	// given
	started := make(chan bool, 1)
	c, _ := newTestChat(t, &mockedAgentBackend{}, WithCommand(blockingCmd{started: started}))
	c.Submit("/block")
	<-started

	// when
	c.Cancel()

	// then
	c.Wait()
	lines := transcript(c)
	require.NotEmpty(t, lines)
	assert.Equal(t, Error, lines[len(lines)-1].Kind)
}

func TestCancelledCompactReportsCancelledInsteadOfFailure(t *testing.T) {
	// given
	started := make(chan bool, 1)
	var c *Chat
	backend := &mockedAgentBackend{
		compactContextFunc: func(ctx context.Context) {
			started <- true
			<-ctx.Done()
			agentFeedback{c}.ContextCompactionFailed()
		},
	}
	c, _ = newTestChat(t, backend, WithCompactCommand())
	c.Submit("/compact")
	<-started

	// when
	c.Cancel()

	// then
	c.Wait()
	assert.Equal(t, []Line{{Kind: Info, Text: "Cancelled."}}, transcript(c))
}

func TestFailedCompactIsReported(t *testing.T) {
	// given
	var c *Chat
	backend := &mockedAgentBackend{
		compactContextFunc: func(context.Context) { agentFeedback{c}.ContextCompactionFailed() },
	}
	c, _ = newTestChat(t, backend, WithCompactCommand())

	// when
	c.Submit("/compact")

	// then
	c.Wait()
	assert.Equal(t, []Line{
		{Kind: Error, Text: "Context compaction failed; will retry after the next turn."},
	}, transcript(c))
}

func TestCloseCancelsWorkAndWaitsForIt(t *testing.T) {
	// given
	started := make(chan bool, 1)
	backend := &mockedAgentBackend{processFunc: blockingUntilCancelled(started)}
	c, _ := newTestChat(t, backend)
	c.Submit("first")
	<-started
	c.Submit("second")

	// when
	c.Close()

	// then
	assert.False(t, c.Busy())
	assert.Equal(t, []string{"first"}, backend.inputs())
}

func TestSubmitAfterCloseIsIgnored(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	c.Close()

	// when
	c.Submit("hello")

	// then
	assert.False(t, c.Busy())
	assert.Empty(t, backend.inputs())
	assert.Empty(t, transcript(c))
}

func TestCloseSilencesTheObserver(t *testing.T) {
	// given
	c, obs := newTestChat(t, &mockedAgentBackend{})
	c.Close()

	// when
	c.append(Info, "late")

	// then
	obs.mu.Lock()
	defer obs.mu.Unlock()
	assert.Zero(t, obs.changes)
}

func TestCloseTwiceIsSafe(t *testing.T) {
	// given
	c, _ := newTestChat(t, &mockedAgentBackend{})
	c.Close()

	// when / then
	assert.NotPanics(t, c.Close)
}

func TestWaitReturnsOnceQueuedWorkHasFinished(t *testing.T) {
	// given
	release := make(chan struct{})
	backend := &mockedAgentBackend{
		processFunc: func(_ context.Context, input string) (*agent.Response, error) {
			<-release
			return &agent.Response{Content: input + " done"}, nil
		},
	}
	c, _ := newTestChat(t, backend)
	c.Submit("first")
	c.Submit("second")
	close(release)

	// when
	c.Wait()

	// then
	lines, _ := c.Next(Cursor{})
	assert.Equal(t, []Line{
		{Kind: User, Text: "first"},
		{Kind: Reply, Text: "first done"},
		{Kind: User, Text: "second"},
		{Kind: Reply, Text: "second done"},
	}, lines)
	assert.False(t, c.Busy())
}

func TestWaitReturnsAtOnceWhenIdle(t *testing.T) {
	// given
	c, _ := newTestChat(t, &mockedAgentBackend{})

	// when / then
	c.Wait()
}
