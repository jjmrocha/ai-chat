package chat

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatToolCall(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		args     map[string]any
		expected string
	}{
		{
			name:     "no arguments",
			tool:     "list",
			args:     nil,
			expected: "list()",
		},
		{
			name:     "arguments are sorted by name",
			tool:     "read",
			args:     map[string]any{"path": "a.go", "limit": 10},
			expected: `read(limit=10, path="a.go")`,
		},
		{
			name:     "nil value renders as null",
			tool:     "x",
			args:     map[string]any{"v": nil},
			expected: "x(v=null)",
		},
		{
			name:     "boolean value",
			tool:     "x",
			args:     map[string]any{"v": true},
			expected: "x(v=true)",
		},
		{
			name:     "oversized string collapses to a size",
			tool:     "x",
			args:     map[string]any{"v": strings.Repeat("a", maxToolArgLen+1)},
			expected: "x(v=<201 B>)",
		},
		{
			name:     "map collapses to a key count when oversized",
			tool:     "x",
			args:     map[string]any{"v": map[string]any{"k": strings.Repeat("b", maxToolArgLen+1)}},
			expected: "x(v={1 key})",
		},
		{
			name:     "small map is encoded",
			tool:     "x",
			args:     map[string]any{"v": map[string]any{"k": 1}},
			expected: `x(v={"k":1})`,
		},
		{
			name:     "slice collapses to a length when oversized",
			tool:     "x",
			args:     map[string]any{"v": []any{strings.Repeat("c", maxToolArgLen+1)}},
			expected: "x(v=[1])",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			tool, args := tc.tool, tc.args

			// when
			result := formatToolCall(tool, args)

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPlural(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		expected string
	}{
		{name: "zero is plural", n: 0, expected: "0 keys"},
		{name: "one is singular", n: 1, expected: "1 key"},
		{name: "many is plural", n: 5, expected: "5 keys"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			result := plural(tc.n, "key")

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

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
	c.mu.Lock()
	meta := c.lastMeta
	cached := c.statusCache
	c.mu.Unlock()
	assert.Equal(t, 120, meta.TotalTokens)
	assert.Nil(t, cached)
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
