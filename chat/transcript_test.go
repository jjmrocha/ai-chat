package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextReturnsOnlyUnseenLines(t *testing.T) {
	// given
	c, _ := newTestChat(t, &mockedAgentBackend{})
	c.append(Info, "one")
	_, cur := c.Next(Cursor{})
	c.append(Info, "two")

	// when
	lines, _ := c.Next(cur)

	// then
	assert.Equal(t, []Line{{Kind: Info, Text: "two"}}, lines)
}

func TestNextWithNothingNewReturnsNoLines(t *testing.T) {
	// given
	c, _ := newTestChat(t, &mockedAgentBackend{})
	c.append(Info, "one")
	_, cur := c.Next(Cursor{})

	// when
	lines, next := c.Next(cur)

	// then
	assert.Empty(t, lines)
	assert.Equal(t, cur, next)
}

func TestNextAfterClearStartsFromTheNewTranscript(t *testing.T) {
	// given
	c, _ := newTestChat(t, &mockedAgentBackend{})
	c.append(Info, "a")
	_, cur := c.Next(Cursor{})
	_ = c.clear()
	c.append(Info, "cleared")
	c.append(User, "hi")

	// when
	lines, _ := c.Next(cur)

	// then
	assert.Equal(t, []Line{
		{Kind: Info, Text: "cleared"},
		{Kind: User, Text: "hi"},
	}, lines)
}

func TestNextReturnsACopy(t *testing.T) {
	// given
	c, _ := newTestChat(t, &mockedAgentBackend{})
	c.append(Info, "one")
	lines, _ := c.Next(Cursor{})

	// when
	lines[0].Text = "changed"

	// then
	assert.Equal(t, "one", c.Transcript()[0].Text)
}

func TestTranscriptStripsTerminalControlSequences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "OSC 52 clipboard write", input: "a\x1b]52;c;Zm9v\x07b", expected: "ab"},
		{name: "OSC 8 hyperlink", input: "\x1b]8;;http://evil\x1b\\link\x1b]8;;\x1b\\", expected: "link"},
		{name: "CSI sequence", input: "a\x1b[2Jb", expected: "ab"},
		{name: "C1 control", input: "a\u009b31mb", expected: "a31mb"},
		{name: "bare C0 controls", input: "a\x07b\x00c\rd", expected: "abcd"},
		{name: "newlines and tabs kept", input: "one\n\ttwo", expected: "one\n\ttwo"},
		{name: "unicode kept", input: "é ❯ 日本", expected: "é ❯ 日本"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			c, _ := newTestChat(t, &mockedAgentBackend{})

			// when
			c.append(Reply, tc.input)

			// then
			assert.Equal(t, tc.expected, c.Transcript()[0].Text)
		})
	}
}

func TestToolCallStripsControlSequencesFromNamesAndKeys(t *testing.T) {
	// given
	c, _ := newTestChat(t, &mockedAgentBackend{})

	// when
	agentFeedback{c}.ToolCalled("t\x1b]0;x\x07ool", map[string]any{"k\x1b]52;c;Zm9v\x07ey": "v"})

	// then
	assert.Equal(t, `tool(key="v")`, c.Progress().Detail)
}

func TestToolResultDetailIsSanitized(t *testing.T) {
	// given
	c, _ := newTestChat(t, &mockedAgentBackend{})
	agentFeedback{c}.ToolCalled("tool", nil)

	// when
	agentFeedback{c}.ToolReturned("tool", "ok\u009b", nil, 0)

	// then
	assert.NotContains(t, c.Transcript()[0].Detail, "\u009b")
}
