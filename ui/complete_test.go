package ui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/jjmrocha/ai-chat/command"
)

func matchNames(c completer) []string {
	names := make([]string, len(c.matches))
	for i, row := range c.matches {
		names[i] = row.name
	}
	return names
}

func TestCompleterOpensOnlyForASingleWordSlashPrefix(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedOpen bool
	}{
		{name: "empty input", input: "", expectedOpen: false},
		{name: "plain text", input: "hi", expectedOpen: false},
		{name: "slash in the middle", input: "hi /cl", expectedOpen: false},
		{name: "bare slash", input: "/", expectedOpen: true},
		{name: "matching prefix", input: "/cl", expectedOpen: true},
		{name: "upper-case prefix", input: "/CL", expectedOpen: true},
		{name: "full name", input: "/clear", expectedOpen: true},
		{name: "arguments started", input: "/clear x", expectedOpen: false},
		{name: "trailing space", input: "/clear ", expectedOpen: false},
		{name: "multi-line", input: "/cl\nx", expectedOpen: false},
		{name: "no match", input: "/zzz", expectedOpen: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			c := newCompleter(stubCommands("clear", "compact", "help"))

			// when
			c.update(tt.input)

			// then
			assert.Equal(t, tt.expectedOpen, c.open())
		})
	}
}

func TestCompleterMatchesByCaseInsensitivePrefixInOrder(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{name: "bare slash lists everything", input: "/", expected: []string{"clear", "compact", "help", "model"}},
		{name: "shared prefix", input: "/c", expected: []string{"clear", "compact"}},
		{name: "upper case", input: "/CO", expected: []string{"compact"}},
		{name: "prefix only, not substring", input: "/el", expected: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			c := newCompleter(stubCommands("clear", "compact", "help", "model"))

			// when
			c.update(tt.input)

			// then
			assert.Equal(t, tt.expected, matchNames(c))
		})
	}
}

func TestCompleterSelectsTheFirstMatch(t *testing.T) {
	// given
	c := newCompleter(stubCommands("clear", "compact"))

	// when
	c.update("/c")

	// then
	assert.Equal(t, "clear", c.selected())
}

func TestCompleterSelectionClampsAtBothEnds(t *testing.T) {
	// given
	c := newCompleter(stubCommands("clear", "compact"))
	c.update("/c")

	// when
	c.prev()
	atTop := c.selected()
	c.next()
	c.next()
	atBottom := c.selected()

	// then
	assert.Equal(t, "clear", atTop)
	assert.Equal(t, "compact", atBottom)
}

func TestCompleterResetsSelectionWhenInputChanges(t *testing.T) {
	// given
	c := newCompleter(stubCommands("clear", "compact", "help"))
	c.update("/")
	c.next()

	// when
	c.update("/c")

	// then
	assert.Equal(t, "clear", c.selected())
}

func TestCompleterKeepsSelectionWhenInputIsUnchanged(t *testing.T) {
	// given
	c := newCompleter(stubCommands("clear", "compact"))
	c.update("/c")
	c.next()

	// when
	c.update("/c")

	// then
	assert.Equal(t, "compact", c.selected())
}

func TestCompleterScrollsToKeepTheSelectionVisible(t *testing.T) {
	// given
	names := make([]string, 12)
	for i := range names {
		names[i] = fmt.Sprintf("cmd%02d", i)
	}
	c := newCompleter(stubCommands(names...))
	c.update("/")

	// when
	for range 9 {
		c.next()
	}
	rows, sel := c.visible()

	// then
	assert.Len(t, rows, maxCompletionRows)
	assert.Equal(t, "cmd09", rows[sel].name)
	assert.Equal(t, fmt.Sprintf("cmd%02d", 9-maxCompletionRows+1), rows[0].name)
}

func TestCompleterScrollsBackUpWithTheSelection(t *testing.T) {
	// given
	names := make([]string, 12)
	for i := range names {
		names[i] = fmt.Sprintf("cmd%02d", i)
	}
	c := newCompleter(stubCommands(names...))
	c.update("/")
	for range 11 {
		c.next()
	}

	// when
	for range 10 {
		c.prev()
	}
	rows, sel := c.visible()

	// then
	assert.Len(t, rows, maxCompletionRows)
	assert.Equal(t, "cmd01", rows[sel].name)
	assert.Equal(t, "cmd01", rows[0].name)
}

func TestCompleterStaysClosedUntilInputChanges(t *testing.T) {
	// given
	c := newCompleter(stubCommands("clear"))
	c.update("/cl")
	c.close()

	// when
	c.update("/cl")
	stillClosed := !c.open()
	c.update("/c")

	// then
	assert.True(t, stillClosed)
	assert.True(t, c.open())
}

func TestCompleterWithNoCommandsNeverOpens(t *testing.T) {
	// given
	c := newCompleter(nil)

	// when
	c.update("/")

	// then
	assert.False(t, c.open())
}

func completionModel(t *testing.T, core *mockedChatCore) model {
	t.Helper()
	if core.commandsFunc == nil {
		core.commandsFunc = func() []command.Command {
			return stubCommands("clear", "compact", "effort", "help", "model")
		}
	}
	return sized(t, core, 80, 24)
}

func typeText(m model, text string) model {
	for _, r := range text {
		next, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = next.(model)
	}
	return m
}

func press(m model, code rune) model {
	next, _ := m.Update(tea.KeyPressMsg{Code: code})
	return next.(model)
}

func TestTypingSlashOpensTheCompletionList(t *testing.T) {
	// given
	m := completionModel(t, &mockedChatCore{})

	// when
	result := typeText(m, "/c")

	// then
	assert.True(t, result.completer.open())
	assert.Equal(t, []string{"clear", "compact"}, matchNames(result.completer))
}

func TestTabFillsTheSelectedCommand(t *testing.T) {
	// given
	m := typeText(completionModel(t, &mockedChatCore{}), "/c")
	m = press(m, tea.KeyDown)

	// when
	result := press(m, tea.KeyTab)

	// then
	assert.Equal(t, "/compact ", result.input.Value())
	assert.False(t, result.completer.open())
}

func TestEnterRunsTheSelectedCommand(t *testing.T) {
	// given
	var submitted []string
	core := &mockedChatCore{submitFunc: func(text string) { submitted = append(submitted, text) }}
	m := typeText(completionModel(t, core), "/c")
	m = press(m, tea.KeyDown)

	// when
	result := press(m, tea.KeyEnter)

	// then
	assert.Equal(t, []string{"/compact"}, submitted)
	assert.Empty(t, result.input.Value())
	assert.False(t, result.completer.open())
}

func TestEnterOnAnExactMatchRunsIt(t *testing.T) {
	// given
	var submitted []string
	core := &mockedChatCore{submitFunc: func(text string) { submitted = append(submitted, text) }}
	m := typeText(completionModel(t, core), "/clear")

	// when
	press(m, tea.KeyEnter)

	// then
	assert.Equal(t, []string{"/clear"}, submitted)
}

func TestRunCommandIsRememberedInHistory(t *testing.T) {
	// given
	m := typeText(completionModel(t, &mockedChatCore{}), "/cl")
	m = press(m, tea.KeyEnter)

	// when
	result := press(m, tea.KeyUp)

	// then
	assert.Equal(t, "/clear", result.input.Value())
}

func TestTabDoesNotSubmit(t *testing.T) {
	// given
	var submitted []string
	core := &mockedChatCore{submitFunc: func(text string) { submitted = append(submitted, text) }}
	m := typeText(completionModel(t, core), "/cl")

	// when
	press(m, tea.KeyTab)

	// then
	assert.Empty(t, submitted)
}

func TestEscClosesTheListWithoutCancelling(t *testing.T) {
	// given
	core := &mockedChatCore{}
	m := typeText(completionModel(t, core), "/")
	core.busy.Store(true)

	// when
	result := press(m, tea.KeyEscape)

	// then
	assert.False(t, result.completer.open())
	assert.Equal(t, "/", result.input.Value())
	assert.Zero(t, core.cancels.Load())
}

func TestSecondEscCancelsTheRunningTurn(t *testing.T) {
	// given
	core := &mockedChatCore{}
	m := press(typeText(completionModel(t, core), "/"), tea.KeyEscape)
	core.busy.Store(true)

	// when
	press(m, tea.KeyEscape)

	// then
	assert.Equal(t, int32(1), core.cancels.Load())
}

func TestArrowsMoveTheSelectionInsteadOfRecallingHistory(t *testing.T) {
	// given
	m := completionModel(t, &mockedChatCore{})
	m.remember("older prompt")
	m = typeText(m, "/")

	// when
	m = press(m, tea.KeyDown)
	m = press(m, tea.KeyDown)
	result := press(m, tea.KeyUp)

	// then
	assert.Equal(t, "/", result.input.Value())
	assert.Equal(t, "compact", result.completer.selected())
}

func TestRecalledSlashCommandKeepsWalkingHistory(t *testing.T) {
	// given
	m := completionModel(t, &mockedChatCore{})
	m.remember("older prompt")
	m.remember("/help")

	// when
	m = press(m, tea.KeyUp)
	recalled := m.input.Value()
	result := press(m, tea.KeyUp)

	// then
	assert.Equal(t, "/help", recalled)
	assert.Equal(t, "older prompt", result.input.Value())
	assert.False(t, result.completer.open())
}

func TestTypingAfterRecallReopensTheList(t *testing.T) {
	// given
	m := completionModel(t, &mockedChatCore{})
	m.remember("/help")
	m = press(m, tea.KeyUp)

	// when
	result := press(m, tea.KeyBackspace)

	// then
	assert.Equal(t, "/hel", result.input.Value())
	assert.True(t, result.completer.open())
}
