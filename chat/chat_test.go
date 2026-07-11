package chat

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/agent"
)

type fakeRunner struct {
	resp  *agent.Response
	err   error
	calls atomic.Int64
}

func (f *fakeRunner) Process(ctx context.Context, input string) (*agent.Response, error) {
	f.calls.Add(1)
	return f.resp, f.err
}

type countObserver struct{ n atomic.Int64 }

func (o *countObserver) TranscriptChanged() { o.n.Add(1) }

func newTestChat(r agentRunner) *Chat { return &Chat{name: "T", agent: r} }

func TestProcessAppendsUserThenReply(t *testing.T) {
	c := newTestChat(&fakeRunner{resp: &agent.Response{
		Content:  "hi there",
		Metadata: agent.Metadata{OutputTokens: 5},
	}})

	c.process(context.Background(), "hello")

	got := c.Transcript()
	if len(got) != 2 {
		t.Fatalf("want 2 lines, got %d: %+v", len(got), got)
	}
	if got[0].Kind != command.User || !strings.Contains(got[0].Text, "hello") {
		t.Errorf("user line = %+v", got[0])
	}
	if got[1].Kind != command.Reply || got[1].Text != "hi there" {
		t.Errorf("reply line = %+v", got[1])
	}
	if c.LastMetadata().OutputTokens != 5 {
		t.Errorf("lastMeta.OutputTokens = %d, want 5", c.LastMetadata().OutputTokens)
	}
}

func TestProcessAppendsErrorOnFailure(t *testing.T) {
	c := newTestChat(&fakeRunner{err: context.DeadlineExceeded})

	c.process(context.Background(), "hello")

	got := c.Transcript()
	if len(got) != 2 {
		t.Fatalf("want 2 lines, got %d: %+v", len(got), got)
	}
	if got[1].Kind != command.Error || !strings.Contains(got[1].Text, context.DeadlineExceeded.Error()) {
		t.Errorf("error line = %+v", got[1])
	}
}

func TestObserverFiresOnEachAppend(t *testing.T) {
	c := newTestChat(&fakeRunner{resp: &agent.Response{Content: "ok"}})
	obs := &countObserver{}
	c.SetObserver(obs)

	c.process(context.Background(), "hello")

	if got := obs.n.Load(); got != 2 { // user line + reply
		t.Errorf("observer fired %d times, want 2", got)
	}
}

func TestSubmitIgnoresBlank(t *testing.T) {
	c := newTestChat(&fakeRunner{resp: &agent.Response{Content: "ok"}})
	c.Submit("   ")
	if got := c.Transcript(); len(got) != 0 {
		t.Errorf("blank submit produced %d lines", len(got))
	}
}

func TestToolCalledAppendsActivity(t *testing.T) {
	c := newTestChat(&fakeRunner{})
	c.ToolCalled("search")
	got := c.Transcript()
	if len(got) != 1 || got[0].Kind != command.Activity || !strings.Contains(got[0].Text, "search") {
		t.Errorf("tool activity = %+v", got)
	}
}

func TestConcurrentAppendsAreSafe(t *testing.T) {
	c := newTestChat(&fakeRunner{})
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			c.ToolCalled("t")
		}()
	}
	wg.Wait()
	if got := len(c.Transcript()); got != n {
		t.Errorf("got %d lines, want %d", got, n)
	}
}
