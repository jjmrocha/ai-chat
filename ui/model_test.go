package ui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	_, cmd := m.Update(spinner.TickMsg{ID: m.spinner.ID()})

	// then
	assert.Nil(t, cmd, "idle tick re-armed the spinner")
}

func TestBusyArmsTheSpinnerOnce(t *testing.T) {
	// given
	core := &mockedChatCore{}
	m := sized(t, core, 80, 24)
	core.busy.Store(true)
	next, first := m.Update(refreshMsg{})
	require.NotNil(t, first, "the first busy event did not arm the spinner")

	// when
	_, second := next.Update(refreshMsg{})

	// then
	assert.Nil(t, second, "a second busy event armed another spinner chain")
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
	next, idleTick := m.Update(spinner.TickMsg{ID: m.spinner.ID()})
	m = next.(model)
	require.Nil(t, idleTick, "the spinner kept running while idle")

	// when
	core.busy.Store(true)
	_, cmd := m.Update(refreshMsg{})

	// then
	assert.NotNil(t, cmd, "the second busy period did not restart the spinner")
}

func TestResizeRedrawsTheTitleBarAtTheNewWidth(t *testing.T) {
	// given
	m := sized(t, &mockedChatCore{}, 40, 24)

	// when
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	// then
	titleBar := liveLines(next.(model))[3]
	assert.Equal(t, 100, lipgloss.Width(titleBar))
	assert.Contains(t, titleBar, "TEST")
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

func TestShiftEnterInsertsANewlinePastTheVisibleHeight(t *testing.T) {
	// given
	m := sized(t, &mockedChatCore{}, 80, 24)
	m.setInput(strings.Repeat("line\n", maxInputLines-1) + "line")

	// when
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})

	// then
	result := next.(model).input
	assert.Equal(t, maxInputLines+1, result.LineCount())
	assert.Equal(t, maxInputLines, result.Height())
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

func TestEscCancelsOnlyWhileBusy(t *testing.T) {
	tests := []struct {
		name            string
		busy            bool
		expectedCancels int32
	}{
		{name: "busy core is cancelled", busy: true, expectedCancels: 1},
		{name: "idle core is left alone", busy: false, expectedCancels: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			core := &mockedChatCore{}
			m := sized(t, core, 80, 24)
			core.busy.Store(tc.busy)

			// when
			m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

			// then
			assert.Equal(t, tc.expectedCancels, core.cancels.Load())
		})
	}
}

func TestEscLeavesTheInputUntouched(t *testing.T) {
	// given
	core := &mockedChatCore{}
	m := sized(t, core, 80, 24)
	core.busy.Store(true)
	m.setInput("draft")

	// when
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	// then
	assert.Equal(t, "draft", next.(model).input.Value())
}
