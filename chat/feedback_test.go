package chat

import (
	"errors"
	"testing"
	"time"

	"github.com/jjmrocha/ai-chat/command"
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

func TestToolCallTextCarriesNoGlyphOrNewline(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	c.ToolCalled("read", nil)

	// when
	c.ToolReturned("read", "ok", nil, time.Millisecond)

	// then
	line := c.Transcript()[0]
	assert.NotContains(t, line.Text, "\n")
	assert.NotContains(t, line.Text, "●")
	assert.NotContains(t, line.Detail, "⎿")
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

func TestToolErrorIsReportedInDetail(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	c.ToolCalled("read", nil)

	// when
	c.ToolReturned("read", "", errors.New("no such file"), time.Millisecond)

	// then
	line := c.Transcript()[0]
	assert.Contains(t, line.Detail, "✗ no such file")
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

func TestContextCompactedReadsLikeTheOtherSessionNotices(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)

	// when
	c.ContextCompacted()

	// then
	lines := c.Transcript()
	require.Len(t, lines, 1)
	assert.Equal(t, command.Info, lines[0].Kind)
	assert.Equal(t, "Context compacted.", lines[0].Text)
	assert.Empty(t, lines[0].Detail)
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

func TestInterimTextIsSanitized(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)

	// when
	c.InterimTextReceived("a\x1b]52;c;Zm9v\x07b\u009b\nc")

	// then
	lines := c.Transcript()
	require.Len(t, lines, 1)
	assert.Equal(t, "ab\nc", lines[0].Text)
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

func TestInterimTextPrecedesToolActivity(t *testing.T) {
	// given
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend)
	c.InterimTextReceived("checking")
	c.ToolCalled("read", nil)

	// when
	c.ToolReturned("read", "ok", nil, time.Millisecond)

	// then
	lines := c.Transcript()
	require.Len(t, lines, 2)
	assert.Equal(t, command.Reply, lines[0].Kind)
	assert.Equal(t, command.Activity, lines[1].Kind)
}
