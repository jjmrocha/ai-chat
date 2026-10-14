package chat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/subagent"
	"github.com/jjmrocha/ai-toolkit/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	answerReply   = `{"message":{"content":"the answer"},"done":true,"prompt_eval_count":20,"eval_count":10}`
	toolCallReply = `{"message":{"content":"","tool_calls":[{"function":{"name":"echo","arguments":{"text":"hi"}}}]},"done":true,"prompt_eval_count":10,"eval_count":5}`
	modelInfo     = `{"model_info":{"general.architecture":"test","test.context_length":8192}}`
)

type modelServer struct {
	mu      sync.Mutex
	replies []string
}

func (s *modelServer) reply() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	reply := s.replies[0]
	if len(s.replies) > 1 {
		s.replies = s.replies[1:]
	}
	return reply
}

func newModelServer(t *testing.T, replies ...string) *llm.LLM {
	t.Helper()
	recorder := &modelServer{replies: replies}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/chat" {
			_, _ = io.WriteString(w, modelInfo)
			return
		}
		_, _ = io.WriteString(w, recorder.reply())
	}))
	t.Cleanup(server.Close)

	model, err := llm.New(llm.Config{Provider: llm.ProviderOllama, Model: "test", BaseURL: server.URL})
	require.NoError(t, err)
	return model
}

func newEchoToolBox(t *testing.T) *tools.ToolBox {
	t.Helper()
	echoTool := llm.Tool{
		Name:        "echo",
		Description: "Echoes text",
		Schema:      tools.NewObjectBuilder().String("text", "the text", true).Build(),
	}
	toolBox := tools.NewToolBox()
	err := toolBox.Add(echoTool, func(_ context.Context, _ map[string]any) (string, error) {
		return "echoed", nil
	})
	require.NoError(t, err)
	return toolBox
}

func newSubagentManager(t *testing.T, replies ...string) *subagent.Manager {
	t.Helper()
	child, err := agent.New(agent.Config{}, newModelServer(t, replies...))
	require.NoError(t, err)
	manager := subagent.NewManager()
	require.NoError(t, manager.Register(subagent.Config{Name: "explore"}, child, newEchoToolBox(t), nil))
	return manager
}

type progressRecorder struct {
	mu    sync.Mutex
	core  *Chat
	texts []string
}

func (r *progressRecorder) TranscriptChanged() {
	text := r.core.Progress().Subagent
	r.mu.Lock()
	r.texts = append(r.texts, text)
	r.mu.Unlock()
}

func (r *progressRecorder) Quit() {}

func (r *progressRecorder) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.texts)
}

func TestWithSubagentsShowsTheRunInProgress(t *testing.T) {
	// given
	manager := newSubagentManager(t, toolCallReply, answerReply)
	c, _ := newTestChat(t, &mockedAgentBackend{}, WithSubagents(manager))
	recorder := &progressRecorder{core: c}
	c.SetObserver(recorder)

	// when
	_, err := manager.Run(t.Context(), "explore", "find the parser")

	// then
	require.NoError(t, err)
	expected := []string{
		"0 tools · 0 tokens",
		"0 tools · 15 tokens",
		"1 tool · 15 tokens",
		"1 tool · 30 tokens",
		"",
	}
	assert.Equal(t, expected, recorder.recorded())
}

func TestWithSubagentsIgnoresANilManager(t *testing.T) {
	// when
	c, _ := newTestChat(t, &mockedAgentBackend{}, WithSubagents(nil))

	// then
	assert.Empty(t, c.Progress().Subagent)
}

func TestSubagentProgressIsEmptyWithoutARun(t *testing.T) {
	// given
	c, _ := newTestChat(t, &mockedAgentBackend{})

	// when
	result := c.Progress().Subagent

	// then
	assert.Empty(t, result)
}

func TestSubagentProgressStartsAtZero(t *testing.T) {
	// given
	c, _ := newTestChat(t, &mockedAgentBackend{})

	// when
	subagentFeedback{c}.SubagentStarted("explore")

	// then
	assert.Equal(t, "0 tools · 0 tokens", c.Progress().Subagent)
}

func TestSubagentProgressCountsToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		calls    int
		expected string
	}{
		{name: "one call", calls: 1, expected: "1 tool · 0 tokens"},
		{name: "two calls", calls: 2, expected: "2 tools · 0 tokens"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			c, _ := newTestChat(t, &mockedAgentBackend{})
			fb := subagentFeedback{c}
			fb.SubagentStarted("explore")

			// when
			for range tc.calls {
				fb.ToolCalled("explore", "echo")
			}

			// then
			assert.Equal(t, tc.expected, c.Progress().Subagent)
		})
	}
}

func TestSubagentProgressShowsTheLatestTokens(t *testing.T) {
	// given
	c, _ := newTestChat(t, &mockedAgentBackend{})
	fb := subagentFeedback{c}
	fb.SubagentStarted("explore")
	fb.TokensUsed("explore", 15)

	// when
	fb.TokensUsed("explore", 12300)

	// then
	assert.Equal(t, "0 tools · 12.30K tokens", c.Progress().Subagent)
}

func TestSubagentProgressClearsWhenTheRunFinishes(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "success", err: nil},
		{name: "error", err: errors.New("boom")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			c, _ := newTestChat(t, &mockedAgentBackend{})
			fb := subagentFeedback{c}
			fb.SubagentStarted("explore")
			fb.ToolCalled("explore", "echo")
			fb.TokensUsed("explore", 15)

			// when
			fb.SubagentFinished("explore", tc.err)

			// then
			assert.Empty(t, c.Progress().Subagent)
		})
	}
}

func TestSubagentProgressResetsOnANewRun(t *testing.T) {
	// given
	c, _ := newTestChat(t, &mockedAgentBackend{})
	fb := subagentFeedback{c}
	fb.SubagentStarted("explore")
	fb.ToolCalled("explore", "echo")
	fb.TokensUsed("explore", 15)
	fb.SubagentFinished("explore", nil)

	// when
	fb.SubagentStarted("explore")

	// then
	assert.Equal(t, "0 tools · 0 tokens", c.Progress().Subagent)
}

func TestSubagentEventsNotifyTheObserver(t *testing.T) {
	tests := []struct {
		name  string
		event func(fb subagentFeedback)
	}{
		{name: "started", event: func(fb subagentFeedback) { fb.SubagentStarted("explore") }},
		{name: "tool called", event: func(fb subagentFeedback) { fb.ToolCalled("explore", "echo") }},
		{name: "tokens used", event: func(fb subagentFeedback) { fb.TokensUsed("explore", 15) }},
		{name: "finished", event: func(fb subagentFeedback) { fb.SubagentFinished("explore", nil) }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			c, obs := newTestChat(t, &mockedAgentBackend{})

			// when
			tc.event(subagentFeedback{c})

			// then
			obs.mu.Lock()
			changes := obs.changes
			obs.mu.Unlock()
			assert.Equal(t, 1, changes)
		})
	}
}
