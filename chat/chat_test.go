package chat

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-chat/theme"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/mcp"
)

type fakeRunner struct {
	resp     *agent.Response
	err      error
	calls    atomic.Int64
	resetErr error
	reset    int
}

func (f *fakeRunner) Process(ctx context.Context, input string) (*agent.Response, error) {
	f.calls.Add(1)
	return f.resp, f.err
}

func (f *fakeRunner) ChangeModel(string) error                   { return nil }
func (f *fakeRunner) ChangeEffort(llm.Effort)                    {}
func (f *fakeRunner) AvailableModels() []string                  { return nil }
func (f *fakeRunner) ModelInfo(context.Context) *agent.ModelInfo { return nil }
func (f *fakeRunner) CompactContext(context.Context)             {}
func (f *fakeRunner) ResetSession() error                        { f.reset++; return f.resetErr }

type countObserver struct {
	n    atomic.Int64
	quit atomic.Int64
}

func (o *countObserver) TranscriptChanged() { o.n.Add(1) }
func (o *countObserver) Quit()              { o.quit.Add(1) }

type signalCmd struct{ done chan string }

func (signalCmd) Name() string                         { return "sig" }
func (signalCmd) Help() string                         { return "/sig            Test command" }
func (s signalCmd) Run(_ command.Context, args string) { s.done <- args }

func newTestChat(r agentBackend) *Chat {
	return &Chat{name: "T", agent: r, ctx: context.Background(), theme: theme.Default}
}

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

func TestThemeDefaultsWhenUnset(t *testing.T) {
	c := newChat("T")
	if c.Theme() != theme.Default {
		t.Errorf("Theme() = %+v, want Default", c.Theme())
	}
}

func TestWithThemeOverridesDefault(t *testing.T) {
	c := newChat("T", WithTheme(theme.Nord))
	if c.Theme() != theme.Nord {
		t.Errorf("Theme() = %+v, want Nord", c.Theme())
	}
}

func TestClearResetsAndEmptiesTranscript(t *testing.T) {
	f := &fakeRunner{}
	c := newTestChat(f)
	c.ToolCalled("x")
	if err := c.Clear(); err != nil {
		t.Fatalf("Clear() error: %v", err)
	}
	if f.reset != 1 {
		t.Errorf("ResetSession called %d times, want 1", f.reset)
	}
	if got := len(c.Transcript()); got != 0 {
		t.Errorf("transcript has %d lines after clear, want 0", got)
	}
}

func TestClearKeepsTranscriptOnResetError(t *testing.T) {
	f := &fakeRunner{resetErr: context.Canceled}
	c := newTestChat(f)
	c.ToolCalled("x")
	if err := c.Clear(); err == nil {
		t.Fatal("Clear() returned nil, want error")
	}
	if got := len(c.Transcript()); got != 1 {
		t.Errorf("transcript has %d lines, want 1 (unchanged)", got)
	}
}

func TestRegistersCommandsFromOptions(t *testing.T) {
	c := newChat("t", WithModelCommand(), WithClearCommand())
	for _, name := range []string{"model", "clear"} {
		if _, ok := c.commands[name]; !ok {
			t.Errorf("command %q not registered", name)
		}
	}
}

func TestWithMCPRegistersMCPCommand(t *testing.T) {
	c := newChat("t", WithMCP(&recMCP{}))
	if _, ok := c.commands["mcp"]; !ok {
		t.Errorf("/mcp not registered by WithMCP")
	}
}

func TestHelpListsRegisteredAndBuiltins(t *testing.T) {
	c := newChat("t", WithModelCommand())
	h := c.helpText()
	for _, want := range []string{"model", "/help", "/exit"} {
		if !strings.Contains(h, want) {
			t.Errorf("help missing %q:\n%s", want, h)
		}
	}
}

func TestDispatchUnknownCommandErrors(t *testing.T) {
	c := newTestChat(&fakeRunner{})
	c.dispatch("/nope")
	got := c.Transcript()
	if len(got) != 1 || got[0].Kind != command.Error || !strings.Contains(got[0].Text, "nope") {
		t.Errorf("unknown command line = %+v", got)
	}
}

func TestDispatchExitSignalsQuit(t *testing.T) {
	c := newTestChat(&fakeRunner{})
	obs := &countObserver{}
	c.SetObserver(obs)
	c.dispatch("/exit")
	if got := obs.quit.Load(); got != 1 {
		t.Errorf("Quit fired %d times, want 1", got)
	}
}

func TestDispatchRunsRegisteredCommandWithArgs(t *testing.T) {
	done := make(chan string, 1)
	c := newChat("t", WithCommand(signalCmd{done: done}))
	c.dispatch("/sig hello world")
	select {
	case args := <-done:
		if args != "hello world" {
			t.Errorf("args = %q, want %q", args, "hello world")
		}
	case <-time.After(time.Second):
		t.Fatal("registered command did not run")
	}
}

type recMCP struct{}

func (recMCP) GetMCPs() []mcp.Status                   { return nil }
func (recMCP) Start(_ context.Context, _ string) error { return nil }
func (recMCP) Stop(_ string) error                     { return nil }

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
