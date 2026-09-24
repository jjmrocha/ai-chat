package ui

import (
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jjmrocha/ai-chat/chat"
)

func sized(t *testing.T, core chatCore, w, h int) model {
	t.Helper()
	m := newModel(core)
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(model)
}

type mockedChatCore struct {
	nameFunc           func() string
	nextFunc           func(cur chat.Cursor) ([]chat.Line, chat.Cursor)
	statusTextFunc     func() string
	queuedFunc         func() bool
	pendingFunc        func() string
	pendingCommandFunc func() string
	submitFunc         func(text string)
	cancellingFunc     func() bool

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

func (m *mockedChatCore) Queued() bool {
	if m.queuedFunc == nil {
		return false
	}
	return m.queuedFunc()
}

func (m *mockedChatCore) PendingTool() string {
	if m.pendingFunc == nil {
		return ""
	}
	return m.pendingFunc()
}

func (m *mockedChatCore) PendingCommand() string {
	if m.pendingCommandFunc == nil {
		return ""
	}
	return m.pendingCommandFunc()
}

func (m *mockedChatCore) Submit(text string) {
	if m.submitFunc != nil {
		m.submitFunc(text)
	}
}

func (m *mockedChatCore) Cancel() { m.cancels.Add(1) }

func (m *mockedChatCore) Cancelling() bool {
	if m.cancellingFunc == nil {
		return false
	}
	return m.cancellingFunc()
}

func linesFrom(all []chat.Line) func(chat.Cursor) ([]chat.Line, chat.Cursor) {
	return func(cur chat.Cursor) ([]chat.Line, chat.Cursor) { return all, cur }
}
