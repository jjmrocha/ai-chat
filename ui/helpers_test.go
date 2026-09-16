package ui

import (
	"sync/atomic"

	"github.com/jjmrocha/ai-chat/chat"
)

type mockedChatCore struct {
	nameFunc           func() string
	transcriptLenFunc  func() int
	sinceFunc          func(n int) []chat.Line
	statusTextFunc     func() string
	queuedFunc         func() bool
	pendingFunc        func() string
	pendingCommandFunc func() string
	submitFunc         func(text string)
	cancellingFunc     func() bool

	busy    atomic.Bool
	cancels atomic.Int32
}

func (m *mockedChatCore) Name() string {
	if m.nameFunc == nil {
		return "TEST"
	}
	return m.nameFunc()
}

func (m *mockedChatCore) TranscriptLen() int {
	if m.transcriptLenFunc == nil {
		return 0
	}
	return m.transcriptLenFunc()
}

func (m *mockedChatCore) Since(n int) []chat.Line {
	if m.sinceFunc == nil {
		return nil
	}
	return m.sinceFunc(n)
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

func linesFrom(all []chat.Line) func(int) []chat.Line {
	return func(n int) []chat.Line {
		if n >= len(all) {
			return nil
		}
		return all[n:]
	}
}
