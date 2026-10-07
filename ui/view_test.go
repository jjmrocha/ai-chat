package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/command"
)

func TestThinkingLineClearsWhenIdle(t *testing.T) {
	// given
	core := &mockedChatCore{}
	m := sized(t, core, 80, 24)
	core.busy.Store(true)
	next, _ := m.Update(refreshMsg{})
	m = next.(model)
	require.NotEmpty(t, m.thinkingLine())

	// when
	core.busy.Store(false)
	next, _ = m.Update(refreshMsg{})

	// then
	assert.Empty(t, next.(model).thinkingLine())
}

func TestTitleBarFallsBackToARuleWhenUnnamed(t *testing.T) {
	// given
	core := &mockedChatCore{nameFunc: func() string { return "" }}

	// when
	m := sized(t, core, 20, 24)

	// then
	assert.Equal(t, m.styles.headerName.Render(m.hrule), m.titleBar)
}

func TestPlaceholderReflectsQueueState(t *testing.T) {
	tests := []struct {
		name     string
		queued   bool
		expected string
	}{
		{name: "idle", queued: false, expected: idlePlaceholder},
		{name: "queued", queued: true, expected: queuedPlaceholder},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			queued := tc.queued
			m := sized(t, &mockedChatCore{progressFunc: func() chat.Progress { return chat.Progress{Queued: queued} }}, 80, 24)

			// when
			result := m.placeholder()

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestThinkingLineShowsThePendingTool(t *testing.T) {
	// given
	core := &mockedChatCore{progressFunc: func() chat.Progress {
		return chat.Progress{Stage: chat.RunningTool, Detail: "read(a.go)"}
	}}
	m := sized(t, core, 80, 24)
	core.busy.Store(true)
	next, _ := m.Update(refreshMsg{})

	// when
	result := next.(model).thinkingLine()

	// then
	assert.Contains(t, result, "read(a.go)")
	assert.NotContains(t, result, "Thinking for")
}

func TestThinkingLineShowsTheRunningCommand(t *testing.T) {
	// given
	core := &mockedChatCore{progressFunc: func() chat.Progress {
		return chat.Progress{Stage: chat.RunningCommand, Detail: "/mcp on yfinance-mcp"}
	}}
	m := sized(t, core, 80, 24)
	core.busy.Store(true)
	next, _ := m.Update(refreshMsg{})

	// when
	result := next.(model).thinkingLine()

	// then
	assert.Contains(t, result, "Waiting for /mcp on yfinance-mcp")
	assert.NotContains(t, result, "Thinking for")
}

func TestViewIsEmptyWhileQuitting(t *testing.T) {
	// given
	m := sized(t, &mockedChatCore{}, 80, 24)
	m.quitting = true

	// when
	result := m.View()

	// then
	assert.Empty(t, strings.TrimSpace(result.Content))
}

func TestThinkingLineShowsCancelling(t *testing.T) {
	// given
	core := &mockedChatCore{progressFunc: func() chat.Progress {
		return chat.Progress{Stage: chat.Cancelling}
	}}
	m := sized(t, core, 80, 24)
	core.busy.Store(true)
	next, _ := m.Update(refreshMsg{})

	// when
	result := next.(model).thinkingLine()

	// then
	assert.Contains(t, result, "Cancelling…")
	assert.NotContains(t, result, "Thinking for")
}

func liveLines(m model) []string {
	lines := strings.Split(ansi.Strip(m.liveRegion()), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return lines
}

func TestLiveRegionReservesThreeRowsWithThinkingInTheMiddle(t *testing.T) {
	// given
	core := &mockedChatCore{}
	m := completionModel(t, core)
	core.busy.Store(true)
	next, _ := m.Update(refreshMsg{})
	m = next.(model)

	// when
	result := liveLines(m)

	// then
	assert.Equal(t, "", result[0])
	assert.Contains(t, result[1], "Thinking")
	assert.Equal(t, "", result[2])
	assert.Equal(t, ansi.Strip(m.titleBar), result[3])
}

func TestLiveRegionKeepsThreeBlankRowsWhenIdle(t *testing.T) {
	// given
	m := typeText(completionModel(t, &mockedChatCore{}), "hi")

	// when
	result := liveLines(m)

	// then
	expected := []string{"", "", "", ansi.Strip(m.titleBar)}
	assert.Equal(t, expected, result[:4])
}

func TestLiveRegionShowsCompletionAboveTheTitleBar(t *testing.T) {
	// given
	m := typeText(completionModel(t, &mockedChatCore{}), "/c")

	// when
	result := liveLines(m)

	// then
	expected := []string{
		"",
		"› /clear    clear help",
		"  /compact  compact help",
		ansi.Strip(m.titleBar),
	}
	assert.Equal(t, expected, result[:4])
}

func TestCompletionReplacesTheThinkingRowsWhileOpen(t *testing.T) {
	// given
	core := &mockedChatCore{}
	m := completionModel(t, core)
	core.busy.Store(true)
	next, _ := m.Update(refreshMsg{})
	m = next.(model)
	withoutList := len(liveLines(m))

	// when
	m = typeText(m, "/")
	result := liveLines(m)

	// then
	assert.Len(t, result, withoutList)
	assert.Equal(t, "› /clear    clear help", result[0])
	assert.NotContains(t, strings.Join(result, "\n"), "Thinking")
}

func TestThinkingLineReturnsWhenTheListCloses(t *testing.T) {
	// given
	core := &mockedChatCore{}
	m := completionModel(t, core)
	core.busy.Store(true)
	next, _ := m.Update(refreshMsg{})
	m = typeText(next.(model), "/c")

	// when
	m = press(m, tea.KeyEscape)
	result := liveLines(m)

	// then
	assert.Contains(t, result[1], "Thinking")
}

func TestCompletionRowsAreTruncatedToTheTerminalWidth(t *testing.T) {
	// given
	core := &mockedChatCore{commandsFunc: func() []command.Command {
		return []command.Command{stubCommand{name: "compact", help: "Summarise the conversation to free context"}}
	}}
	m := typeText(completionModel(t, core), "/")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 24})
	m = next.(model)

	// when
	result := strings.Split(m.completionView(), "\n")

	// then
	for _, row := range result {
		if row == "" {
			continue
		}
		assert.LessOrEqual(t, lipgloss.Width(row), 20)
		assert.True(t, strings.HasSuffix(ansi.Strip(row), "…"))
	}
}
