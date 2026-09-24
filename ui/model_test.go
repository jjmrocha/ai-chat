package ui

import (
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
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
