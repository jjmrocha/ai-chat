package ui

import (
	"sync/atomic"

	"github.com/jjmrocha/ai-chat/chat"
)

type mockedChatCore struct {
	nameFunc          func() string
	transcriptLenFunc func() int
	sinceFunc         func(n int) []chat.Line
	statusTextFunc    func() string
	queuedFunc        func() bool
	pendingFunc       func() string
	submitFunc        func(text string)

	busy atomic.Bool
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

func (m *mockedChatCore) Submit(text string) {
	if m.submitFunc != nil {
		m.submitFunc(text)
	}
}

func linesFrom(all []chat.Line) func(int) []chat.Line {
	return func(n int) []chat.Line {
		if n >= len(all) {
			return nil
		}
		return all[n:]
	}
}
