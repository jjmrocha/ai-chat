package ui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jjmrocha/ai-chat/chat"
)

type chatCore interface {
	Name() string
	Next(cur chat.Cursor) ([]chat.Line, chat.Cursor)
	Busy() bool
	StatusText() string
	Queued() bool
	PendingTool() string
	PendingCommand() string
	Submit(text string)
	Cancel()
	Cancelling() bool
}

type (
	refreshMsg struct{}
	quitMsg    struct{}

	printedMsg struct{}
)

type observer struct{ program *tea.Program }

func (o *observer) TranscriptChanged() {
	if o.program != nil {
		go o.program.Send(refreshMsg{})
	}
}

func (o *observer) Quit() {
	if o.program != nil {
		go o.program.Send(quitMsg{})
	}
}

// Run renders core in the terminal and blocks until the user quits with Ctrl+C
// or /exit, or until ctx is cancelled.
//
// It installs itself as the core's observer and sets the core's context, so
// there is no need to call chat.Chat.SetObserver or chat.Chat.SetContext
// beforehand. Before returning it closes core with chat.Chat.Close, cancelling
// any turn still running and waiting for it, so the agent is idle once Run
// returns. It returns the Bubble Tea program's error, or nil on a clean exit.
func Run(ctx context.Context, core *chat.Chat) error {
	core.SetContext(ctx)
	p := tea.NewProgram(newModel(core), tea.WithContext(ctx))
	core.SetObserver(&observer{program: p})
	defer core.Close()
	_, err := p.Run()
	return err
}
