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
	mu                 sync.Mutex
	processFunc        func(ctx context.Context, input string) (*agent.Response, error)
	compactContextFunc func(ctx context.Context)
	resetSessionFunc   func() error
	messages           []llm.Message

	processed []string
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

func (m *mockedAgentBackend) Messages() []llm.Message {
	return m.messages
}

func (m *mockedAgentBackend) inputs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.processed...)
}

type mockedModelBackend struct {
	mu              sync.Mutex
	info            llm.ModelInfo
	effort          llm.Effort
	models          []string
	changeModelErr  error
	changeEffortErr error

	changing chan struct{}
	release  chan struct{}
}

func (m *mockedModelBackend) ModelInfo() *llm.ModelInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	info := m.info
	return &info
}

func (m *mockedModelBackend) Effort() llm.Effort {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.effort
}

func (m *mockedModelBackend) AvailableModels() []string {
	return m.models
}

func (m *mockedModelBackend) ChangeModel(_ context.Context, model string) (*llm.ModelInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.changing != nil {
		m.changing <- struct{}{}
		<-m.release
	}
	if m.changeModelErr != nil {
		return nil, m.changeModelErr
	}
	m.info.Name = model
	info := m.info
	return &info, nil
}

func (m *mockedModelBackend) ChangeEffort(e llm.Effort) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.changeEffortErr != nil {
		return m.changeEffortErr
	}
	m.effort = e
	return nil
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
	return newTestChatWithModel(t, backend, &mockedModelBackend{}, opts...)
}

func newTestChatWithModel(t *testing.T, backend *mockedAgentBackend, model *mockedModelBackend, opts ...Option) (*Chat, *mockedObserver) {
	t.Helper()
	c := newChat("TEST", model, opts...)
	c.agent = backend
	c.now = newFakeClock().current
	obs := &mockedObserver{}
	c.SetObserver(obs)
	return c, obs
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
}

func (f *fakeClock) current() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	f.mu.Unlock()
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
