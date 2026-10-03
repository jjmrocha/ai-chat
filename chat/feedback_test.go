package chat

import (
	"testing"
	"time"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolCallAndReturnBecomeOneLine(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	c.ToolCalled("read", map[string]any{"path": "a.go"})

	// when
	c.ToolReturned("read", "contents", nil, 3*time.Millisecond)

	// then
	lines := c.Transcript()
	require.Len(t, lines, 1)
	assert.Equal(t, command.Activity, lines[0].Kind)
	assert.Equal(t, `read(path="a.go")`, lines[0].Text)
	assert.Equal(t, "contents · 3ms", lines[0].Detail)
}

func TestToolReturnedWithoutPendingCallIsDropped(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)

	// when
	c.ToolReturned("read", "ok", nil, time.Millisecond)

	// then
	assert.Zero(t, c.TranscriptLen())
}

func TestTokensUsedUpdatesStatus(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, obs := newTestChat(t, backend)

	// when
	c.TokensUsed(120)

	// then
	assert.Equal(t, 120, c.Status().Tokens)
	obs.mu.Lock()
	changes := obs.changes
	obs.mu.Unlock()
	assert.Equal(t, 1, changes)
}

func TestPendingToolIsClearedAfterReturn(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	c.ToolCalled("read", nil)
	require.NotEmpty(t, c.PendingTool())

	// when
	c.ToolReturned("read", "ok", nil, time.Millisecond)

	// then
	assert.Empty(t, c.PendingTool())
}

func TestSessionResumedReplaysTheConversationBeforeTheNotice(t *testing.T) {
	// given
	backend := &mockedAgentBackend{messages: []llm.Message{
		llm.UserMessage{Content: "first question"},
		llm.AssistantMessage{Content: "first answer"},
		llm.UserMessage{Content: "second question"},
		llm.AssistantMessage{Content: "second answer"},
	}}
	c, _ := newTestChat(t, backend)

	// when
	c.SessionResumed("abc")

	// then
	assert.Equal(t, []Line{
		{Kind: command.User, Text: "first question"},
		{Kind: command.Reply, Text: "first answer"},
		{Kind: command.User, Text: "second question"},
		{Kind: command.Reply, Text: "second answer"},
		{Kind: command.Info, Text: "Session abc resumed."},
	}, c.Transcript())
}

func TestSessionResumedSkipsToolTurns(t *testing.T) {
	// given
	backend := &mockedAgentBackend{messages: []llm.Message{
		llm.UserMessage{Content: "read it"},
		llm.AssistantMessage{
			Content:   "reading",
			ToolCalls: []llm.ToolCall{{ID: "1", Name: "read_file"}},
		},
		llm.ToolMessage{ToolCallID: "1", ToolName: "read_file", Content: "data"},
		llm.AssistantMessage{Content: "done"},
	}}
	c, _ := newTestChat(t, backend)

	// when
	c.SessionResumed("abc")

	// then
	assert.Equal(t, []Line{
		{Kind: command.User, Text: "read it"},
		{Kind: command.Reply, Text: "done"},
		{Kind: command.Info, Text: "Session abc resumed."},
	}, c.Transcript())
}

func TestSessionResumedShowsTheLastResponseTokens(t *testing.T) {
	// given
	backend := &mockedAgentBackend{messages: []llm.Message{
		llm.UserMessage{Content: "read it"},
		llm.AssistantMessage{Content: "first", Stats: llm.Stats{TotalTokens: 100}},
		llm.UserMessage{Content: "again"},
		llm.AssistantMessage{
			ToolCalls: []llm.ToolCall{{ID: "1", Name: "read_file"}},
			Stats:     llm.Stats{TotalTokens: 250},
		},
		llm.ToolMessage{ToolCallID: "1", ToolName: "read_file", Content: "data"},
	}}
	c, _ := newTestChat(t, backend)

	// when
	c.SessionResumed("abc")

	// then
	assert.Equal(t, 250, c.Status().Tokens)
}

func TestTurnClosesAnUnreturnedToolCall(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	c.ToolCalled("hang", nil)

	// when
	c.Submit("go")
	waitIdle(t, c)

	// then
	var activity []Line
	for _, ln := range c.Transcript() {
		if ln.Kind == command.Activity {
			activity = append(activity, ln)
		}
	}
	require.Len(t, activity, 1)
	assert.Equal(t, "hang()", activity[0].Text)
	assert.Equal(t, "(no result)", activity[0].Detail)
	assert.Empty(t, c.PendingTool())
}

func TestInterimTextBecomesReplyLine(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)

	// when
	c.InterimTextReceived("Let me check the config.")

	// then
	lines := c.Transcript()
	require.Len(t, lines, 1)
	assert.Equal(t, command.Reply, lines[0].Kind)
	assert.Equal(t, "Let me check the config.", lines[0].Text)
}

func TestBlankInterimTextIsDropped(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "empty", content: ""},
		{name: "whitespace only", content: "  \n\t"},
		{name: "escape sequences only", content: "\x1b[31m\x1b[0m"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			backend := &mockedAgentBackend{}
			c, _ := newTestChat(t, backend)

			// when
			c.InterimTextReceived(tc.content)

			// then
			assert.Zero(t, c.TranscriptLen())
		})
	}
}
