package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-chat/theme"
)

type mockedChatCore struct {
	nameFunc       func() string
	transcriptFunc func() []chat.Line
	busyFunc       func() bool
	statusTextFunc func() string
	submitFunc     func(text string)
}

func (m *mockedChatCore) Name() string {
	if m.nameFunc == nil {
		return "TEST"
	}
	return m.nameFunc()
}

func (m *mockedChatCore) Theme() theme.Theme { return theme.Default }

func (m *mockedChatCore) Transcript() []chat.Line {
	if m.transcriptFunc == nil {
		return nil
	}
	return m.transcriptFunc()
}

func (m *mockedChatCore) Busy() bool {
	if m.busyFunc == nil {
		return false
	}
	return m.busyFunc()
}

func (m *mockedChatCore) StatusText() string {
	if m.statusTextFunc == nil {
		return ""
	}
	return m.statusTextFunc()
}

func (m *mockedChatCore) Submit(text string) {
	if m.submitFunc != nil {
		m.submitFunc(text)
	}
}

func sizedModel(t *testing.T, core chatCore) model {
	t.Helper()
	m := newModel(core)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	sized, ok := updated.(model)
	if !ok {
		t.Fatalf("Update returned %T, expected model", updated)
	}
	return sized
}

func TestRenderBlock(t *testing.T) {
	tests := []struct {
		name string
		kind command.Kind
	}{
		{name: "user line", kind: command.User},
		{name: "info line", kind: command.Info},
		{name: "error line", kind: command.Error},
		{name: "activity line", kind: command.Activity},
		{name: "telemetry line", kind: command.Telemetry},
		{name: "reply line", kind: command.Reply},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			m := sizedModel(t, &mockedChatCore{})
			line := chat.Line{Kind: tc.kind, Text: "payload"}

			// when
			result := m.renderBlock(line)

			// then
			assert.Contains(t, result, "payload")
		})
	}
}

func TestTitleBar(t *testing.T) {
	t.Run("with name", func(t *testing.T) {
		// given
		m := sizedModel(t, &mockedChatCore{nameFunc: func() string { return "MYCHAT" }})

		// when
		result := m.titleBar()

		// then
		assert.Contains(t, result, "MYCHAT")
		assert.Contains(t, result, "─")
	})

	t.Run("without name", func(t *testing.T) {
		// given
		m := sizedModel(t, &mockedChatCore{nameFunc: func() string { return "" }})

		// when
		result := m.titleBar()

		// then
		assert.NotEmpty(t, result)
		assert.NotContains(t, result, " ")
	})
}

func TestModelView(t *testing.T) {
	t.Run("before first window size", func(t *testing.T) {
		// given
		m := newModel(&mockedChatCore{})

		// when
		result := m.View()

		// then
		assert.Contains(t, result.Content, "Initializing…")
	})

	t.Run("renders transcript and status", func(t *testing.T) {
		// given
		core := &mockedChatCore{
			transcriptFunc: func() []chat.Line {
				return []chat.Line{{Kind: command.Info, Text: "hello there"}}
			},
			statusTextFunc: func() string { return "model-x · ctx:0%" },
		}
		m := sizedModel(t, core)

		// when
		result := m.View()

		// then
		assert.NotContains(t, result.Content, "hello there", "transcript belongs to the scrollback now")
		assert.Contains(t, result.Content, "model-x")
	})

	t.Run("the live region keeps the title bar above the input", func(t *testing.T) {
		// given
		core := &mockedChatCore{
			nameFunc:       func() string { return "MYCHAT" },
			statusTextFunc: func() string { return "model-x" },
		}
		m := sizedModel(t, core)

		// when
		result := m.View()

		// then
		assert.Contains(t, result.Content, "MYCHAT")
		assert.Less(t, strings.Index(result.Content, "MYCHAT"), strings.Index(result.Content, "model-x"),
			"the name sits above the status line")
	})

	t.Run("a rendered reply carries no newline of its own", func(t *testing.T) {
		// given
		m := sizedModel(t, &mockedChatCore{})

		// when
		result := m.renderMarkdown("Hello there")

		// then
		assert.False(t, strings.HasPrefix(result, "\n"), "the block spacing is added once, by pending")
		assert.False(t, strings.HasSuffix(result, "\n"))
	})

	t.Run("quitting leaves nothing behind", func(t *testing.T) {
		tests := []struct {
			name string
			msg  tea.Msg
		}{
			{name: "ctrl+c", msg: tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}},
			{name: "core quit", msg: quitMsg{}},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				// given
				core := &mockedChatCore{
					nameFunc:       func() string { return "MYCHAT" },
					statusTextFunc: func() string { return "model-x" },
				}
				m := sizedModel(t, core)

				// when
				updated, cmd := m.Update(tc.msg)

				// then
				result, ok := updated.(model)
				require.True(t, ok)
				assert.Empty(t, result.View().Content, "the live region is wiped on the way out")
				assert.NotNil(t, cmd)
			})
		}
	})

	t.Run("the input is indented instead of marked", func(t *testing.T) {
		// given
		m := sizedModel(t, &mockedChatCore{})

		// then
		assert.Equal(t, inputIndent, m.input.Prompt)
	})

	t.Run("the input carries no prompt marker", func(t *testing.T) {
		// given
		m := sizedModel(t, &mockedChatCore{})
		m.input.SetValue("typed text")

		// when
		result := m.input.View()

		// then
		for _, marker := range []string{"❯", "┃", ">"} {
			assert.NotContains(t, result, marker)
		}
	})

	t.Run("a rule closes the live region between input and status", func(t *testing.T) {
		// given
		core := &mockedChatCore{
			nameFunc:       func() string { return "MYCHAT" },
			statusTextFunc: func() string { return "model-x" },
		}
		m := sizedModel(t, core)

		// when
		lines := strings.Split(m.View().Content, "\n")

		// then
		require.GreaterOrEqual(t, len(lines), 2)
		rule := lines[len(lines)-2]
		assert.Contains(t, rule, "─")
		assert.Contains(t, lines[len(lines)-1], "model-x")
	})

	t.Run("view is inline so the terminal keeps selection and scrollback", func(t *testing.T) {
		// given
		m := sizedModel(t, &mockedChatCore{})

		// when
		result := m.View()

		// then
		assert.False(t, result.AltScreen)
		assert.Equal(t, tea.MouseModeNone, result.MouseMode)
	})

	t.Run("busy shows spinner instead of status", func(t *testing.T) {
		// given
		core := &mockedChatCore{
			busyFunc:       func() bool { return true },
			statusTextFunc: func() string { return "model-x" },
		}
		m := sizedModel(t, core)

		// when
		result := m.View()

		// then
		assert.Contains(t, result.Content, "thinking…")
		assert.NotContains(t, result.Content, "model-x")
	})
}

func TestModelUpdate(t *testing.T) {
	t.Run("enter submits input to the core", func(t *testing.T) {
		// given
		var submitted string
		core := &mockedChatCore{submitFunc: func(text string) { submitted = text }}
		m := sizedModel(t, core)
		m.input.SetValue("hello agent")

		// when
		updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		// then
		assert.Equal(t, "hello agent", submitted)
		result, ok := updated.(model)
		require.True(t, ok)
		assert.Empty(t, result.input.Value())
	})

	t.Run("shift+enter inserts a newline instead of submitting", func(t *testing.T) {
		// given
		submitted := false
		core := &mockedChatCore{submitFunc: func(string) { submitted = true }}
		m := sizedModel(t, core)
		m.input.SetValue("first")

		// when
		updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})

		// then
		assert.False(t, submitted, "shift+enter must not submit")
		result, ok := updated.(model)
		require.True(t, ok)
		assert.Equal(t, "first\n", result.input.Value())
	})

	t.Run("enter submits a multi-line value intact", func(t *testing.T) {
		// given
		var submitted string
		core := &mockedChatCore{submitFunc: func(text string) { submitted = text }}
		m := sizedModel(t, core)
		m.input.SetValue("line one\nline two")

		// when
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		// then
		assert.Equal(t, "line one\nline two", submitted)
	})

	t.Run("input stops growing at the cap", func(t *testing.T) {
		// given
		m := sizedModel(t, &mockedChatCore{})

		// when
		m.input.SetValue(strings.Repeat("x\n", 20))
		updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})

		// then
		result, ok := updated.(model)
		require.True(t, ok)
		assert.Equal(t, maxInputLines, result.input.Height())
	})

	t.Run("up walks back through previous prompts", func(t *testing.T) {
		// given
		m := submitAll(t, "first", "second")

		// when / then
		m = press(t, m, tea.KeyUp)
		assert.Equal(t, "second", m.input.Value())

		m = press(t, m, tea.KeyUp)
		assert.Equal(t, "first", m.input.Value())

		m = press(t, m, tea.KeyUp)
		assert.Equal(t, "first", m.input.Value(), "stops at the oldest prompt")
	})

	t.Run("down walks forward and restores the stashed draft", func(t *testing.T) {
		// given
		m := submitAll(t, "first", "second")
		m.input.SetValue("half typed")

		// when / then
		m = press(t, m, tea.KeyUp)
		assert.Equal(t, "second", m.input.Value())

		m = press(t, m, tea.KeyUp)
		assert.Equal(t, "first", m.input.Value())

		m = press(t, m, tea.KeyDown)
		assert.Equal(t, "second", m.input.Value())

		m = press(t, m, tea.KeyDown)
		assert.Equal(t, "half typed", m.input.Value(), "the draft comes back untouched")
	})

	t.Run("up inside a multi-line draft moves the cursor, not history", func(t *testing.T) {
		// given
		m := submitAll(t, "first")
		m.input.SetValue("draft line one\ndraft line two")
		require.Equal(t, 1, m.input.Line(), "cursor starts on the last line")

		// when
		m = press(t, m, tea.KeyUp)

		// then
		assert.Equal(t, "draft line one\ndraft line two", m.input.Value())
		assert.Equal(t, 0, m.input.Line(), "cursor moved up instead")
	})

	t.Run("up from the top line of a multi-line draft recalls history", func(t *testing.T) {
		// given
		m := submitAll(t, "first")
		m.input.SetValue("draft line one\ndraft line two")
		m = press(t, m, tea.KeyUp)

		// when
		m = press(t, m, tea.KeyUp)

		// then
		assert.Equal(t, "first", m.input.Value())
	})

	t.Run("down with no history browsing moves the cursor", func(t *testing.T) {
		// given
		m := submitAll(t, "first")
		m.input.SetValue("a\nb")
		m = press(t, m, tea.KeyUp)
		require.Equal(t, 0, m.input.Line())

		// when
		m = press(t, m, tea.KeyDown)

		// then
		assert.Equal(t, "a\nb", m.input.Value())
		assert.Equal(t, 1, m.input.Line())
	})

	t.Run("blank submissions are not recorded", func(t *testing.T) {
		// given
		m := submitAll(t, "first", "   ", "")

		// when
		m = press(t, m, tea.KeyUp)

		// then
		assert.Equal(t, "first", m.input.Value())
	})

	t.Run("consecutive duplicates are recorded once", func(t *testing.T) {
		// given
		m := submitAll(t, "first", "same", "same")

		// when
		m = press(t, m, tea.KeyUp)
		m = press(t, m, tea.KeyUp)

		// then
		assert.Equal(t, "first", m.input.Value())
	})

	t.Run("submitting ends history browsing", func(t *testing.T) {
		// given
		m := submitAll(t, "first", "second")
		m = press(t, m, tea.KeyUp)
		require.Equal(t, "second", m.input.Value())

		// when
		m = press(t, m, tea.KeyEnter)
		m = press(t, m, tea.KeyUp)

		// then
		assert.Equal(t, "second", m.input.Value(), "recall restarts from the newest prompt")
	})

	t.Run("only unprinted transcript lines are emitted", func(t *testing.T) {
		// given
		lines := []chat.Line{{Kind: command.Info, Text: "first"}}
		core := &mockedChatCore{transcriptFunc: func() []chat.Line { return lines }}
		m := sizedModel(t, core)
		m = refreshed(t, m)
		require.Equal(t, 1, m.printed)

		// when
		lines = append(lines, chat.Line{Kind: command.Info, Text: "second"})
		pending := m.pending()

		// then
		require.Len(t, pending, 1)
		assert.Contains(t, pending[0], "second")
		assert.NotContains(t, pending[0], "first")
		assert.True(t, strings.HasPrefix(pending[0], "\n"), "a blank line opens each block")
		assert.True(t, strings.HasSuffix(pending[0], "\n"), "and another closes it")
	})

	t.Run("emitting advances the printed count and returns a command", func(t *testing.T) {
		// given
		var lines []chat.Line
		core := &mockedChatCore{transcriptFunc: func() []chat.Line { return lines }}
		m := sizedModel(t, core)
		require.Zero(t, m.printed)
		lines = append(lines, chat.Line{Kind: command.Info, Text: "first"})

		// when
		updated, cmd := m.Update(refreshMsg{})

		// then
		result, ok := updated.(model)
		require.True(t, ok)
		assert.Equal(t, 1, result.printed)
		assert.NotNil(t, cmd, "the new line has to reach the terminal")
	})

	t.Run("nothing new emits no command", func(t *testing.T) {
		// given
		lines := []chat.Line{{Kind: command.Info, Text: "first"}}
		core := &mockedChatCore{transcriptFunc: func() []chat.Line { return lines }}
		m := refreshed(t, sizedModel(t, core))

		// when
		updated, cmd := m.Update(refreshMsg{})

		// then
		result, ok := updated.(model)
		require.True(t, ok)
		assert.Equal(t, 1, result.printed)
		assert.Nil(t, cmd)
	})

	t.Run("a reset session reprints what replaced the old lines", func(t *testing.T) {
		// given
		lines := []chat.Line{
			{Kind: command.Info, Text: "old one"},
			{Kind: command.Info, Text: "old two"},
		}
		core := &mockedChatCore{transcriptFunc: func() []chat.Line { return lines }}
		m := refreshed(t, sizedModel(t, core))
		require.Equal(t, 2, m.printed)

		// when: /clear empties the transcript, then prints its confirmation
		lines = []chat.Line{{Kind: command.Info, Text: "Context cleared."}}
		updated, cmd := m.Update(refreshMsg{})

		// then
		result, ok := updated.(model)
		require.True(t, ok)
		assert.Equal(t, 1, result.printed, "the confirmation is printed, not dropped")
		assert.NotNil(t, cmd)
	})

	t.Run("a shrinking transcript restarts the count and leaves the screen alone", func(t *testing.T) {
		// given
		lines := []chat.Line{{Kind: command.Info, Text: "old line"}}
		core := &mockedChatCore{transcriptFunc: func() []chat.Line { return lines }}
		m := refreshed(t, sizedModel(t, core))
		require.Equal(t, 1, m.printed)

		// when
		lines = nil
		updated, cmd := m.Update(refreshMsg{})

		// then
		result, ok := updated.(model)
		require.True(t, ok)
		assert.Zero(t, result.printed)
		assert.Nil(t, cmd, "nothing to print and nothing to wipe")
	})

}

func TestObserver(t *testing.T) {
	t.Run("nil program is safe", func(t *testing.T) {
		// given
		o := &observer{}

		// when
		o.TranscriptChanged()
		o.Quit()

		// then
		assert.Nil(t, o.program)
	})
}

func TestHrule(t *testing.T) {
	// given
	m := sizedModel(t, &mockedChatCore{})

	// when
	result := m.hrule()

	// then
	expected := strings.Repeat("─", 80)
	assert.Equal(t, expected, result)
}

// press sends one key to the model and returns the updated model.
func press(t *testing.T, m model, code rune) model {
	t.Helper()
	updated, _ := m.Update(tea.KeyPressMsg{Code: code})
	result, ok := updated.(model)
	require.True(t, ok)
	return result
}

// submitAll builds a sized model and submits each text in turn.
func submitAll(t *testing.T, texts ...string) model {
	t.Helper()
	m := sizedModel(t, &mockedChatCore{})
	for _, text := range texts {
		m.input.SetValue(text)
		m = press(t, m, tea.KeyEnter)
	}
	return m
}

// refreshed drives one refresh cycle and returns the resulting model.
func refreshed(t *testing.T, m model) model {
	t.Helper()
	updated, _ := m.Update(refreshMsg{})
	result, ok := updated.(model)
	require.True(t, ok)
	return result
}
