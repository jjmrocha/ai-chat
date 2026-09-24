package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			m := sized(t, &mockedChatCore{queuedFunc: func() bool { return queued }}, 80, 24)

			// when
			result := m.placeholder()

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestThinkingLineShowsThePendingTool(t *testing.T) {
	// given
	core := &mockedChatCore{pendingFunc: func() string { return "read(a.go)" }}
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
	core := &mockedChatCore{pendingCommandFunc: func() string { return "/mcp on yfinance-mcp" }}
	m := sized(t, core, 80, 24)
	core.busy.Store(true)
	next, _ := m.Update(refreshMsg{})

	// when
	result := next.(model).thinkingLine()

	// then
	assert.Contains(t, result, "Waiting for /mcp on yfinance-mcp")
	assert.NotContains(t, result, "Thinking for")
}

func TestThinkingLineShowsThePendingToolOverTheRunningCommand(t *testing.T) {
	// given
	core := &mockedChatCore{
		pendingFunc:        func() string { return "read(a.go)" },
		pendingCommandFunc: func() string { return "/mcp on yfinance-mcp" },
	}
	m := sized(t, core, 80, 24)
	core.busy.Store(true)
	next, _ := m.Update(refreshMsg{})

	// when
	result := next.(model).thinkingLine()

	// then
	assert.Contains(t, result, "read(a.go)")
	assert.NotContains(t, result, "Waiting for")
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

func TestThinkingLineShowsCancellingOverThePendingTool(t *testing.T) {
	// given
	core := &mockedChatCore{
		pendingFunc:    func() string { return "read(a.go)" },
		cancellingFunc: func() bool { return true },
	}
	m := sized(t, core, 80, 24)
	core.busy.Store(true)
	next, _ := m.Update(refreshMsg{})

	// when
	result := next.(model).thinkingLine()

	// then
	assert.Contains(t, result, "Cancelling…")
	assert.NotContains(t, result, "read(a.go)")
	assert.NotContains(t, result, "Thinking for")
}

func TestLiveRegionKeepsTheFullWidthRule(t *testing.T) {
	// given
	core := &mockedChatCore{}

	// when
	m := sized(t, core, 80, 24)

	// then
	assert.Equal(t, strings.Repeat("─", 80), m.hrule)
}
