package chat

import (
	"context"
	"testing"
	"time"

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
	ctx.Print(command.Error, "Error: "+ctx.Context().Err().Error())
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
	waitIdle(t, c)
	lines := c.Transcript()
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
	waitIdle(t, c)
	lines := c.Transcript()
	require.NotEmpty(t, lines)
	assert.Equal(t, command.Error, lines[len(lines)-1].Kind)
}

func TestCompactUsesTheCommandContext(t *testing.T) {
	// given
	started := make(chan bool, 1)
	backend := &mockedAgentBackend{
		compactContextFunc: func(ctx context.Context) {
			started <- true
			<-ctx.Done()
		},
	}
	c, _ := newTestChat(t, backend, WithCompactCommand())
	c.Submit("/compact")
	<-started

	// when
	c.Cancel()

	// then
	waitIdle(t, c)
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

func TestCloseCancelsAPendingModelLookup(t *testing.T) {
	// given
	started := make(chan bool, 1)
	backend := &mockedAgentBackend{
		modelInfoFunc: func(ctx context.Context) *agent.ModelInfo {
			started <- true
			<-ctx.Done()
			return nil
		},
	}
	c, _ := newTestChat(t, backend)
	c.Status()
	<-started

	// when
	done := make(chan struct{})
	go func() { c.Close(); close(done) }()

	// then
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return")
	}
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
	assert.Empty(t, c.Transcript())
}

func TestCloseSilencesTheObserver(t *testing.T) {
	// given
	c, obs := newTestChat(t, &mockedAgentBackend{})
	c.Close()

	// when
	c.Print(command.Info, "late")

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
