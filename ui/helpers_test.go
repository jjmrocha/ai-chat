package ui

import (
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/command"
)

func sized(t *testing.T, core chatCore, w, h int) model {
	t.Helper()
	m := newModel(core)
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(model)
}

type mockedChatCore struct {
	nameFunc       func() string
	nextFunc       func(cur chat.Cursor) ([]chat.Line, chat.Cursor)
	statusTextFunc func() string
	progressFunc   func() chat.Progress
	submitFunc     func(text string)
	commandsFunc   func() []command.Command

	busy      atomic.Bool
	cancels   atomic.Int32
	nextCalls atomic.Int32
}

func (m *mockedChatCore) Name() string {
	if m.nameFunc == nil {
		return "TEST"
	}
	return m.nameFunc()
}

func (m *mockedChatCore) Next(cur chat.Cursor) ([]chat.Line, chat.Cursor) {
	m.nextCalls.Add(1)
	if m.nextFunc == nil {
		return nil, cur
	}
	return m.nextFunc(cur)
}

func (m *mockedChatCore) Busy() bool { return m.busy.Load() }

func (m *mockedChatCore) StatusText() string {
	if m.statusTextFunc == nil {
		return ""
	}
	return m.statusTextFunc()
}

func (m *mockedChatCore) Progress() chat.Progress {
	if m.progressFunc == nil {
		return chat.Progress{}
	}
	return m.progressFunc()
}

func (m *mockedChatCore) Submit(text string) {
	if m.submitFunc != nil {
		m.submitFunc(text)
	}
}

func (m *mockedChatCore) Cancel() { m.cancels.Add(1) }

func (m *mockedChatCore) Commands() []command.Command {
	if m.commandsFunc == nil {
		return nil
	}
	return m.commandsFunc()
}

type stubCommand struct {
	name string
	help string
}

func (s stubCommand) Name() string                { return s.name }
func (s stubCommand) Help() string                { return s.help }
func (s stubCommand) Run(command.Context, string) {}

func stubCommands(names ...string) []command.Command {
	cmds := make([]command.Command, len(names))
	for i, name := range names {
		cmds[i] = stubCommand{name: name, help: name + " help"}
	}
	return cmds
}

func linesFrom(all []chat.Line) func(chat.Cursor) ([]chat.Line, chat.Cursor) {
	return func(cur chat.Cursor) ([]chat.Line, chat.Cursor) { return all, cur }
}
