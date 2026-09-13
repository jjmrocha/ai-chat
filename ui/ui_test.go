package ui

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/command"
)

func sized(t *testing.T, core chatCore, w, h int) model {
	t.Helper()
	m := newModel(core)
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(model)
}

func TestInitDoesNotStartTheSpinner(t *testing.T) {
	// given
	core := &mockedChatCore{}
	m := newModel(core)

	// when
	result := m.Init()

	// then
	require.NotNil(t, result)
	_, isTick := result().(spinner.TickMsg)
	assert.False(t, isTick, "Init armed a spinner tick while idle")
}

func TestIdleTickStopsTheChain(t *testing.T) {
	// given
	core := &mockedChatCore{}
	m := sized(t, core, 80, 24)

	// when
	next, cmd := m.Update(spinner.TickMsg{ID: m.spinner.ID()})

	// then
	assert.Nil(t, cmd, "idle tick re-armed the spinner")
	assert.False(t, next.(model).spinning)
}

func TestBusyArmsTheSpinnerOnce(t *testing.T) {
	// given
	core := &mockedChatCore{}
	m := sized(t, core, 80, 24)
	core.busy.Store(true)

	// when
	next, cmd := m.Update(refreshMsg{})

	// then
	m = next.(model)
	assert.True(t, m.spinning)
	assert.NotNil(t, cmd)

	// a second event while already spinning must not arm a second chain
	next, _ = m.Update(refreshMsg{})
	assert.True(t, next.(model).spinning)
}

func TestBusyTickKeepsTheChainAlive(t *testing.T) {
	// given
	core := &mockedChatCore{}
	m := sized(t, core, 80, 24)
	core.busy.Store(true)
	next, _ := m.Update(refreshMsg{})
	m = next.(model)

	// when
	_, cmd := m.Update(spinner.TickMsg{ID: m.spinner.ID()})

	// then
	assert.NotNil(t, cmd, "busy tick did not re-arm the chain")
}

func TestSpinnerRestartsOnASecondBusyPeriod(t *testing.T) {
	// given
	core := &mockedChatCore{}
	m := sized(t, core, 80, 24)
	core.busy.Store(true)
	next, _ := m.Update(refreshMsg{})
	m = next.(model)

	core.busy.Store(false)
	next, _ = m.Update(spinner.TickMsg{ID: m.spinner.ID()})
	m = next.(model)
	require.False(t, m.spinning)

	// when
	core.busy.Store(true)
	next, cmd := m.Update(refreshMsg{})

	// then
	assert.True(t, next.(model).spinning)
	assert.NotNil(t, cmd)
}

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

func TestRenderBlockAppliesGlyphs(t *testing.T) {
	tests := []struct {
		name     string
		line     chat.Line
		contains []string
		absent   []string
	}{
		{
			name:     "user line gains the prompt glyph",
			line:     chat.Line{Kind: command.User, Text: "hello"},
			contains: []string{"❯ hello"},
		},
		{
			name:     "activity with detail gains both glyphs",
			line:     chat.Line{Kind: command.Activity, Text: "read(x)", Detail: "ok"},
			contains: []string{"● read(x)", "⎿ ok"},
		},
		{
			name:     "activity without detail omits the detail glyph",
			line:     chat.Line{Kind: command.Activity, Text: "context compacted"},
			contains: []string{"● context compacted"},
			absent:   []string{"⎿"},
		},
		{
			name:     "info line is unadorned",
			line:     chat.Line{Kind: command.Info, Text: "note"},
			contains: []string{"note"},
			absent:   []string{"❯", "●", "⎿"},
		},
		{
			name:     "error line is unadorned",
			line:     chat.Line{Kind: command.Error, Text: "boom"},
			contains: []string{"boom"},
			absent:   []string{"❯", "●"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			m := sized(t, &mockedChatCore{}, 80, 24)

			// when
			result := m.renderBlock(tc.line)

			// then
			for _, want := range tc.contains {
				assert.Contains(t, result, want)
			}
			for _, unwanted := range tc.absent {
				assert.NotContains(t, result, unwanted)
			}
		})
	}
}

func TestRenderBlockStylesMarkdownReplies(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		contains    []string
		absent      []string
		sgrParams   []string
		noSgrParams []string
	}{
		{
			name:        "inline emphasis renders as attributes, not markup",
			text:        "**bold** *italic* ~~gone~~",
			contains:    []string{"bold", "italic", "gone"},
			absent:      []string{"**", "~~", "*italic*"},
			sgrParams:   []string{"1", "3", "9"},
			noSgrParams: []string{"38", "48"},
		},
		{
			name:        "inline code drops the backticks and uses the info color",
			text:        "run `make test` now",
			contains:    []string{"make test"},
			absent:      []string{"`"},
			sgrParams:   []string{"36"},
			noSgrParams: []string{"38", "48"},
		},
		{
			name:        "link keeps text and URL, text underlined in the info color",
			text:        "see [docs](https://example.com)",
			contains:    []string{"docs", "https://example.com"},
			absent:      []string{"](", "[docs"},
			sgrParams:   []string{"4", "36", "90"},
			noSgrParams: []string{"38", "48"},
		},
		{
			name:        "fenced code is highlighted with basic colors only",
			text:        "```go\nfunc main() {} // hi\n```",
			contains:    []string{"func", "// hi"},
			absent:      []string{"```"},
			sgrParams:   []string{"31", "90"},
			noSgrParams: []string{"38", "48"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			m := sized(t, &mockedChatCore{}, 80, 24)

			// when
			result := m.renderBlock(chat.Line{Kind: command.Reply, Text: tc.text})

			// then
			params := sgrParams(result)
			for _, want := range tc.contains {
				assert.Contains(t, stripSGR(result), want)
			}
			for _, unwanted := range tc.absent {
				assert.NotContains(t, stripSGR(result), unwanted)
			}
			for _, want := range tc.sgrParams {
				assert.Truef(t, params[want], "SGR parameter %s missing in %q", want, result)
			}
			for _, unwanted := range tc.noSgrParams {
				assert.Falsef(t, params[unwanted], "SGR parameter %s present in %q", unwanted, result)
			}
		})
	}
}

var sgrPattern = regexp.MustCompile("\x1b\\[([0-9;]*)m")

func sgrParams(s string) map[string]bool {
	params := map[string]bool{}
	for _, match := range sgrPattern.FindAllStringSubmatch(s, -1) {
		for _, p := range strings.Split(match[1], ";") {
			params[p] = true
		}
	}
	return params
}

func stripSGR(s string) string {
	return sgrPattern.ReplaceAllString(s, "")
}

func TestResizeRecomputesTheCachedChrome(t *testing.T) {
	// given
	m := sized(t, &mockedChatCore{}, 40, 24)
	require.Len(t, []rune(m.hrule), 40)
	first := m.titleBar

	// when
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	// then
	m = next.(model)
	assert.Len(t, []rune(m.hrule), 100)
	assert.NotEqual(t, first, m.titleBar)
}

func TestTitleBarFallsBackToARuleWhenUnnamed(t *testing.T) {
	// given
	core := &mockedChatCore{nameFunc: func() string { return "" }}

	// when
	m := sized(t, core, 20, 24)

	// then
	assert.Equal(t, m.styles.headerName.Render(m.hrule), m.titleBar)
}

func TestPendingRendersOnlyTheUnprintedTail(t *testing.T) {
	// given
	all := []chat.Line{
		{Kind: command.Info, Text: "one"},
		{Kind: command.Info, Text: "two"},
		{Kind: command.Info, Text: "three"},
	}
	core := &mockedChatCore{
		transcriptLenFunc: func() int { return len(all) },
		sinceFunc:         linesFrom(all),
	}
	m := sized(t, core, 80, 24)
	m.printed = 2

	// when
	result := m.pending()

	// then
	require.Len(t, result, 1)
	assert.Contains(t, result[0], "three")
}

func TestEmitResetsWhenTranscriptShrinks(t *testing.T) {
	// given
	core := &mockedChatCore{
		transcriptLenFunc: func() int { return 0 },
		sinceFunc:         func(int) []chat.Line { return nil },
	}
	m := sized(t, core, 80, 24)
	m.printed = 5

	// when
	next, _ := m.emit()

	// then
	assert.Zero(t, next.(model).printed)
}

func TestEmitIsSuppressedWhilePrinting(t *testing.T) {
	// given
	all := []chat.Line{{Kind: command.Info, Text: "one"}}
	core := &mockedChatCore{
		transcriptLenFunc: func() int { return len(all) },
		sinceFunc:         linesFrom(all),
	}
	m := sized(t, core, 80, 24)
	m.printing = true
	before := m.printed

	// when
	next, cmd := m.emit()

	// then
	assert.Nil(t, cmd)
	assert.Equal(t, before, next.(model).printed)
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

func TestInputHistoryRecall(t *testing.T) {
	t.Run("recall walks backwards then forwards", func(t *testing.T) {
		// given
		m := sized(t, &mockedChatCore{}, 80, 24)
		m.remember("first")
		m.remember("second")

		// when / then
		require.True(t, m.recallOlder())
		assert.Equal(t, "second", m.input.Value())

		require.True(t, m.recallOlder())
		assert.Equal(t, "first", m.input.Value())

		require.True(t, m.recallNewer())
		assert.Equal(t, "second", m.input.Value())
	})

	t.Run("recall past the oldest entry is refused", func(t *testing.T) {
		// given
		m := sized(t, &mockedChatCore{}, 80, 24)
		m.remember("only")
		require.True(t, m.recallOlder())

		// when
		result := m.recallOlder()

		// then
		assert.False(t, result)
	})

	t.Run("empty history has nothing to recall", func(t *testing.T) {
		// given
		m := sized(t, &mockedChatCore{}, 80, 24)

		// when
		result := m.recallOlder()

		// then
		assert.False(t, result)
	})

	t.Run("blank input is not remembered", func(t *testing.T) {
		// given
		m := sized(t, &mockedChatCore{}, 80, 24)

		// when
		m.remember("   ")

		// then
		assert.Empty(t, m.history)
	})

	t.Run("consecutive duplicates are collapsed", func(t *testing.T) {
		// given
		m := sized(t, &mockedChatCore{}, 80, 24)

		// when
		m.remember("same")
		m.remember("same")

		// then
		assert.Len(t, m.history, 1)
	})

	t.Run("returning to the newest restores the draft", func(t *testing.T) {
		// given
		m := sized(t, &mockedChatCore{}, 80, 24)
		m.remember("sent")
		m.setInput("half typed")
		require.True(t, m.recallOlder())
		require.Equal(t, "sent", m.input.Value())

		// when
		result := m.recallNewer()

		// then
		assert.True(t, result)
		assert.Equal(t, "half typed", m.input.Value())
	})
}

func TestShiftEnterInsertsANewlineInsteadOfSubmitting(t *testing.T) {
	// given
	var submitted []string
	core := &mockedChatCore{submitFunc: func(text string) { submitted = append(submitted, text) }}
	m := sized(t, core, 80, 24)
	m.setInput("first")

	// when
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})

	// then
	assert.Empty(t, submitted)
	assert.Equal(t, "first\n", next.(model).input.Value())
}

func TestCtrlCQuits(t *testing.T) {
	// given
	m := sized(t, &mockedChatCore{}, 80, 24)

	// when
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	// then
	assert.True(t, next.(model).quitting)
	assert.NotNil(t, cmd)
}

func TestQuitMsgQuits(t *testing.T) {
	// given
	m := sized(t, &mockedChatCore{}, 80, 24)

	// when
	next, cmd := m.Update(quitMsg{})

	// then
	assert.True(t, next.(model).quitting)
	assert.NotNil(t, cmd)
}

func TestEnterSubmitsAndClearsInput(t *testing.T) {
	// given
	var submitted []string
	core := &mockedChatCore{submitFunc: func(text string) { submitted = append(submitted, text) }}
	m := sized(t, core, 80, 24)
	m.setInput("hello")

	// when
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// then
	assert.Equal(t, []string{"hello"}, submitted)
	assert.Empty(t, next.(model).input.Value())
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
