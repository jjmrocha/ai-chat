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
		assert.Contains(t, result.Content, "hello there")
		assert.Contains(t, result.Content, "model-x")
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

	t.Run("refresh picks up new transcript lines", func(t *testing.T) {
		// given
		lines := []chat.Line{}
		core := &mockedChatCore{transcriptFunc: func() []chat.Line { return lines }}
		m := sizedModel(t, core)
		lines = append(lines, chat.Line{Kind: command.Info, Text: "appended later"})

		// when
		updated, _ := m.Update(refreshMsg{})

		// then
		result, ok := updated.(model)
		require.True(t, ok)
		assert.Contains(t, result.View().Content, "appended later")
	})

	t.Run("clear shrinks the transcript", func(t *testing.T) {
		// given
		lines := []chat.Line{{Kind: command.Info, Text: "old line"}}
		core := &mockedChatCore{transcriptFunc: func() []chat.Line { return lines }}
		m := sizedModel(t, core)
		updated, _ := m.Update(refreshMsg{})
		filled, ok := updated.(model)
		if !ok {
			t.Fatalf("Update returned %T, expected model", updated)
		}
		lines = nil

		// when
		updated, _ = filled.Update(refreshMsg{})

		// then
		result, ok := updated.(model)
		require.True(t, ok)
		assert.NotContains(t, result.View().Content, "old line")
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
