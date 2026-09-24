package chat

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
)

type mockedAgentBackend struct {
	mu                  sync.Mutex
	processFunc         func(ctx context.Context, input string) (*agent.Response, error)
	changeModelFunc     func(name string) error
	changeEffortFunc    func(e llm.Effort) error
	availableModelsFunc func() []string
	modelInfoFunc       func(ctx context.Context) *agent.ModelInfo
	compactContextFunc  func(ctx context.Context)
	resetSessionFunc    func() error

	processed     []string
	modelInfoHits int
}

func (m *mockedAgentBackend) Process(ctx context.Context, input string) (*agent.Response, error) {
	m.mu.Lock()
	m.processed = append(m.processed, input)
	fn := m.processFunc
	m.mu.Unlock()
	if fn == nil {
		return &agent.Response{Content: "reply"}, nil
	}
	return fn(ctx, input)
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
	m.mu.Lock()
	m.modelInfoHits++
	fn := m.modelInfoFunc
	m.mu.Unlock()
	if fn == nil {
		return nil
	}
	return fn(ctx)
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

func (m *mockedAgentBackend) inputs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.processed...)
}

func (m *mockedAgentBackend) infoHits() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.modelInfoHits
}

type mockedObserver struct {
	mu       sync.Mutex
	changes  int
	quits    int
	quitFunc func()
}

func (m *mockedObserver) TranscriptChanged() {
	m.mu.Lock()
	m.changes++
	m.mu.Unlock()
}

func (m *mockedObserver) Quit() {
	m.mu.Lock()
	m.quits++
	fn := m.quitFunc
	m.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (m *mockedObserver) quitCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.quits
}

func newTestChat(t *testing.T, backend *mockedAgentBackend, opts ...Option) (*Chat, *mockedObserver) {
	t.Helper()
	c := newChat("TEST", opts...)
	c.agent = backend
	obs := &mockedObserver{}
	c.SetObserver(obs)
	return c, obs
}

func waitIdle(t *testing.T, c *Chat) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !c.Busy() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("chat never went idle")
}

type renderingObserver struct{ core *Chat }

func (r *renderingObserver) TranscriptChanged() { r.core.Status() }

func (r *renderingObserver) Quit() {}

func waitStatus(t *testing.T, c *Chat, done func(StatusInfo) bool) StatusInfo {
	t.Helper()
	var last StatusInfo
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if last = c.Status(); done(last) {
			return last
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("status never settled, last: %+v", last)
	return last
}
