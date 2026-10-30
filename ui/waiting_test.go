package ui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const waitingTimeout = 2 * time.Second

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func returnsWithin(t *testing.T, call func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		call()
	}()
	select {
	case <-done:
	case <-time.After(waitingTimeout):
		t.Fatal("call did not return in time")
	}
}

func startTestWaiting(t *testing.T, ctx context.Context, out io.Writer, interrupt func()) (*Waiting, *io.PipeWriter) {
	t.Helper()
	in, keys := io.Pipe()
	t.Cleanup(func() { _ = keys.Close() })
	w := startWaiting(ctx, "Starting serena…", out, true, 80, interrupt,
		tea.WithInput(in),
		tea.WithWindowSize(80, 24),
	)
	return w, keys
}

func TestWaitingStopReturnsOnceTheProgramEnds(t *testing.T) {
	// given
	w, _ := startTestWaiting(t, context.Background(), &syncBuffer{}, nil)

	// when / then
	returnsWithin(t, w.Stop)
}

func TestWaitingDrawsTheTextOnTheOutput(t *testing.T) {
	// given
	out := &syncBuffer{}

	// when
	w, _ := startTestWaiting(t, context.Background(), out, nil)
	defer w.Stop()

	// then
	assert.Eventually(t, func() bool {
		return strings.Contains(ansi.Strip(out.String()), "Starting serena…")
	}, waitingTimeout, 10*time.Millisecond)
}

func TestWaitingSecondStopReturnsImmediately(t *testing.T) {
	// given
	w, _ := startTestWaiting(t, context.Background(), &syncBuffer{}, nil)
	w.Stop()

	// when / then
	returnsWithin(t, w.Stop)
}

func TestWaitingStopsWhenTheContextIsCancelled(t *testing.T) {
	// given
	ctx, cancel := context.WithCancel(context.Background())
	w, _ := startTestWaiting(t, ctx, &syncBuffer{}, nil)

	// when
	cancel()

	// then
	select {
	case <-w.done:
	case <-time.After(waitingTimeout):
		t.Fatal("program still running after the context was cancelled")
	}
}

func TestWaitingWithoutATerminalDrawsNothing(t *testing.T) {
	// given
	out := &syncBuffer{}
	w := startWaiting(context.Background(), "Starting…", out, false, 80, nil)

	// when
	returnsWithin(t, func() {
		w.SetText("Still starting…")
		w.Stop()
		w.Stop()
	})

	// then
	assert.Empty(t, out.String())
}

func TestWaitingViewShowsTheSpinnerThenTheText(t *testing.T) {
	// given
	m := newWaitingModel("Starting serena…", 80)

	// when
	result := m.View()

	// then
	expected := m.spinner.View() + m.style.Render(" Starting serena…")
	assert.Equal(t, expected, result.Content)
}

func TestWaitingNewTextReplacesTheOldOne(t *testing.T) {
	// given
	m := newWaitingModel("Starting serena…", 80)

	// when
	next, _ := m.Update(waitingTextMsg("Starting subagent explorer…"))

	// then
	content := ansi.Strip(next.View().Content)
	assert.Contains(t, content, "Starting subagent explorer…")
	assert.NotContains(t, content, "Starting serena…")
}

func TestWaitingLongTextIsCutAtTheTerminalWidth(t *testing.T) {
	// given
	m := newWaitingModel(strings.Repeat("x", 100), 80)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 24})

	// when
	result := next.View()

	// then
	assert.Equal(t, 20, ansi.StringWidth(result.Content))
	assert.True(t, strings.HasSuffix(ansi.Strip(result.Content), "…"))
}

func drawnWaitingModel(t *testing.T) tea.Model {
	t.Helper()
	var m tea.Model = newWaitingModel("Starting…", 80)
	for range 2 {
		m, _ = m.Update(spinner.TickMsg{ID: m.(waitingModel).spinner.ID()})
	}
	return m
}

func TestWaitingStopEmptiesTheViewAndQuits(t *testing.T) {
	// given
	m := drawnWaitingModel(t)

	// when
	next, cmd := m.Update(waitingStopMsg{})

	// then
	assert.Empty(t, next.View().Content)
	require.NotNil(t, cmd)
	assert.Equal(t, tea.QuitMsg{}, cmd())
}

func TestWaitingStopBeforeTheFirstFrameWaitsForIt(t *testing.T) {
	// given
	m := newWaitingModel("Starting…", 80)

	// when
	next, cmd := m.Update(waitingStopMsg{})

	// then
	assert.NotEmpty(t, next.View().Content)
	assert.Nil(t, cmd)
}

func TestWaitingSetTextAfterStopDoesNotBlock(t *testing.T) {
	// given
	w, _ := startTestWaiting(t, context.Background(), &syncBuffer{}, nil)
	w.Stop()

	// when / then
	returnsWithin(t, func() { w.SetText("too late") })
}

func TestWaitingCtrlCErasesTheLineThenInterrupts(t *testing.T) {
	// given
	interrupted := make(chan struct{})
	w, keys := startTestWaiting(t, context.Background(), &syncBuffer{}, func() { close(interrupted) })

	// when
	_, err := keys.Write([]byte{0x03})

	// then
	require.NoError(t, err)
	select {
	case <-interrupted:
	case <-time.After(waitingTimeout):
		t.Fatal("ctrl+c did not interrupt the app")
	}
	select {
	case <-w.done:
	default:
		t.Fatal("the app was interrupted before the line was erased")
	}
}

func TestWaitingPrintlnRightAfterStartKeepsTheLine(t *testing.T) {
	// given
	out := &syncBuffer{}
	w, _ := startTestWaiting(t, context.Background(), out, nil)

	// when
	w.Println("starting mcp context7: exit status 1")
	w.Stop()

	// then
	assert.Contains(t, ansi.Strip(out.String()), "starting mcp context7: exit status 1")
}

func TestWaitingPrintBeforeTheFirstFrameIsHeldBack(t *testing.T) {
	tests := []struct {
		name  string
		ticks int
	}{
		{name: "no tick yet", ticks: 0},
		{name: "only the tick from Init", ticks: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var m tea.Model = newWaitingModel("Starting…", 80)
			for range tc.ticks {
				m, _ = m.Update(spinner.TickMsg{ID: m.(waitingModel).spinner.ID()})
			}

			// when
			_, cmd := m.Update(waitingPrintMsg("starting mcp context7: exit status 1"))

			// then
			assert.Nil(t, cmd)
		})
	}
}

func TestWaitingPrintAfterTheFirstFrameIsSent(t *testing.T) {
	// given
	m := drawnWaitingModel(t)

	// when
	_, cmd := m.Update(waitingPrintMsg("starting mcp context7: exit status 1"))

	// then
	assert.NotNil(t, cmd)
}

func TestWaitingPrintlnThenStopKeepsTheLine(t *testing.T) {
	// given
	out := &syncBuffer{}
	w, _ := startTestWaiting(t, context.Background(), out, nil)
	require.Eventually(t, func() bool {
		return strings.Contains(ansi.Strip(out.String()), "Starting serena…")
	}, waitingTimeout, 10*time.Millisecond)
	time.Sleep(150 * time.Millisecond)

	// when
	w.Println("starting mcp context7: exit status 1")
	w.Stop()

	// then
	assert.Contains(t, ansi.Strip(out.String()), "starting mcp context7: exit status 1")
}

func TestWaitingPrintlnAfterStopWritesThePlainLine(t *testing.T) {
	// given
	out := &syncBuffer{}
	w, _ := startTestWaiting(t, context.Background(), out, nil)
	w.Stop()
	before := out.String()

	// when
	returnsWithin(t, func() { w.Println("starting mcp context7: exit status 1") })

	// then
	assert.Equal(t, before+"starting mcp context7: exit status 1\n", out.String())
}

func TestWaitingConcurrentPrintlnAndStopPrintsEachLineOnce(t *testing.T) {
	// given
	out := &syncBuffer{}
	w, _ := startTestWaiting(t, context.Background(), out, nil)
	const lines = 20
	var wg sync.WaitGroup

	// when
	for i := range lines {
		wg.Go(func() { w.Println(fmt.Sprintf("concurrent line %02d", i)) })
	}
	wg.Go(w.Stop)
	returnsWithin(t, wg.Wait)

	// then
	content := ansi.Strip(out.String())
	for i := range lines {
		assert.Equal(t, 1, strings.Count(content, fmt.Sprintf("concurrent line %02d", i)), "line %02d", i)
	}
}

func TestWaitingPrintlnWithoutATerminalWritesThePlainLine(t *testing.T) {
	// given
	out := &syncBuffer{}
	w := startWaiting(context.Background(), "Starting…", out, false, 80, nil)

	// when
	w.Println("starting mcp context7: exit status 1")

	// then
	assert.Equal(t, "starting mcp context7: exit status 1\n", out.String())
}

func TestWaitingPrintlnWithEmptyTextWritesNothing(t *testing.T) {
	// given
	out := &syncBuffer{}
	w := startWaiting(context.Background(), "Starting…", out, false, 80, nil)

	// when
	w.Println("")

	// then
	assert.Empty(t, out.String())
}
