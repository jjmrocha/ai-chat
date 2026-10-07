package chat

// Stage is what the core is busy with, as reported by [Chat.Progress].
type Stage int

// The stages a [Progress] may report.
const (
	// Idle means no turn or command is running.
	Idle Stage = iota

	// Thinking means an agent turn is waiting on the model.
	Thinking

	// RunningTool means the agent is running a tool call.
	RunningTool

	// RunningCommand means a slash command is running.
	RunningCommand

	// Cancelling means [Chat.Cancel] was called and the running turn or
	// command has not returned yet.
	Cancelling
)

// Progress is one consistent snapshot of what the core is doing, for a
// front-end's progress row. The zero value is an idle core with nothing queued.
type Progress struct {
	Stage Stage

	// Detail describes the call for [RunningTool] and is the command as typed
	// for [RunningCommand]. It is empty for every other stage.
	Detail string

	// Queued reports that input is waiting behind the running turn or command.
	Queued bool
}

// Progress reports what the core is doing right now. Cancelling takes
// precedence over a tool call, and a tool call over a running command. It never
// waits on the agent, so a front-end may call it on every frame.
func (c *Chat) Progress() Progress {
	c.mu.Lock()
	defer c.mu.Unlock()

	busy, queued, cancelling := c.inbox.state()
	p := Progress{Queued: queued}
	switch {
	case cancelling:
		p.Stage = Cancelling
	case c.pendingTool != "":
		p.Stage, p.Detail = RunningTool, c.pendingTool
	case c.pendingCommand != "":
		p.Stage, p.Detail = RunningCommand, c.pendingCommand
	case busy:
		p.Stage = Thinking
	}
	return p
}
