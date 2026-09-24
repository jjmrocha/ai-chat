package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/command"
)

func TestEmitPrintsTheLinesTheCoreReturns(t *testing.T) {
	// given
	core := &mockedChatCore{nextFunc: linesFrom([]chat.Line{{Kind: command.Info, Text: "one"}})}
	m := sized(t, core, 80, 24)
	m.printing = false

	// when
	next, cmd := m.emit()

	// then
	assert.NotNil(t, cmd)
	assert.True(t, next.(model).printing)
}

func TestEmitIsSuppressedWhilePrinting(t *testing.T) {
	// given
	core := &mockedChatCore{nextFunc: linesFrom([]chat.Line{{Kind: command.Info, Text: "one"}})}
	m := sized(t, core, 80, 24)
	m.printing = true
	core.nextCalls.Store(0)

	// when
	_, cmd := m.emit()

	// then
	assert.Nil(t, cmd)
	assert.Zero(t, core.nextCalls.Load())
}

func TestChunkBlock(t *testing.T) {
	tests := []struct {
		name     string
		block    string
		limit    int
		expected []string
	}{
		{
			name:     "zero limit returns the block whole",
			block:    "a\nb\nc",
			limit:    0,
			expected: []string{"a\nb\nc"},
		},
		{
			name:     "negative limit returns the block whole",
			block:    "a\nb",
			limit:    -1,
			expected: []string{"a\nb"},
		},
		{
			name:     "block under the limit is untouched",
			block:    "a\nb",
			limit:    5,
			expected: []string{"a\nb"},
		},
		{
			name:     "block exactly at the limit is untouched",
			block:    "a\nb",
			limit:    2,
			expected: []string{"a\nb"},
		},
		{
			name:     "block over the limit is split",
			block:    "a\nb\nc",
			limit:    2,
			expected: []string{"a\nb", "c"},
		},
		{
			name:     "split is exact when evenly divisible",
			block:    "a\nb\nc\nd",
			limit:    2,
			expected: []string{"a\nb", "c\nd"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			block, limit := tc.block, tc.limit

			// when
			result := chunkBlock(block, limit)

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func everyKind() []chat.Line {
	return []chat.Line{
		{Kind: command.User, Text: strings.Repeat("typed ", 40)},
		{Kind: command.Info, Text: strings.Repeat("noted ", 40)},
		{Kind: command.Error, Text: "Error: " + strings.Repeat("why ", 40)},
		{
			Kind:   command.Activity,
			Text:   `decision_yes_no(question="` + strings.Repeat("q", 180) + `")`,
			Detail: strings.Repeat("d", 150),
		},
		{Kind: command.Reply, Text: strings.Repeat("word ", 200)},
		{Kind: command.Telemetry, Text: "7 tool calls · 12.3s"},
	}
}

func widestLine(t *testing.T, block string) int {
	t.Helper()
	widest := 0
	for _, line := range strings.Split(block, "\n") {
		widest = max(widest, lipgloss.Width(line))
	}
	return widest
}

func TestPendingKeepsEveryBlockWithinThePrintedWidth(t *testing.T) {
	for _, width := range []int{20, 40, 80, 120} {
		// given
		all := everyKind()
		core := &mockedChatCore{
			nextFunc: linesFrom(all),
		}
		m := sized(t, core, width, 24)

		// when
		blocks, _ := m.pending()

		// then
		require.Len(t, blocks, len(all))
		for i, block := range blocks {
			assert.LessOrEqual(t, widestLine(t, block), m.printWidth(),
				"width %d, block %d (%v) exceeds the printed width", width, i, all[i].Kind)
		}
	}
}

func TestPendingNeverEmitsALineThatFillsTheTerminalWidth(t *testing.T) {
	// given
	all := everyKind()
	core := &mockedChatCore{
		nextFunc: linesFrom(all),
	}
	m := sized(t, core, 80, 24)

	// when
	blocks, _ := m.pending()

	// then
	for i, block := range blocks {
		for _, line := range strings.Split(block, "\n") {
			w := lipgloss.Width(line)
			assert.False(t, w > 0 && w%m.width == 0,
				"block %d (%v) has a line of exactly %d columns, which miscounts on insertAbove", i, all[i].Kind, w)
		}
	}
}

func TestPendingKeepsTheTurnSeparatorOnTheTelemetryBlock(t *testing.T) {
	// given
	all := []chat.Line{{Kind: command.Telemetry, Text: "7 tool calls · 12.3s"}}
	core := &mockedChatCore{
		nextFunc: linesFrom(all),
	}
	m := sized(t, core, 80, 24)

	// when
	blocks, _ := m.pending()

	// then
	require.Len(t, blocks, 1)
	lines := strings.Split(blocks[0], "\n")
	require.Len(t, lines, 3, "the rule must fit one line, with no orphan wrapped onto the next")
	assert.Equal(t, m.printWidth(), lipgloss.Width(lines[1]))
	assert.Contains(t, lines[1], "─")
	assert.Contains(t, lines[2], "7 tool calls · 12.3s")
}

func TestPendingIsUnwrappedBeforeTheFirstResize(t *testing.T) {
	// given
	all := []chat.Line{{Kind: command.Info, Text: strings.Repeat("noted ", 40)}}
	core := &mockedChatCore{
		nextFunc: linesFrom(all),
	}
	m := newModel(core)

	// when
	blocks, _ := m.pending()

	// then
	require.Len(t, blocks, 1)
	assert.Contains(t, blocks[0], strings.Repeat("noted ", 40))
}
