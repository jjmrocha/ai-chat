package chat

import "time"

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

	// Elapsed is how long the current [Thinking], [RunningTool] or
	// [RunningCommand] stage has run. Thinking starts again when a tool call
	// returns and when the next queued input starts. It is zero for every
	// other stage.
	Elapsed time.Duration

	// Queued reports that input is waiting behind the running turn or command.
	Queued bool

	// Subagent describes the running sub-agent's work, for example
	// "3 tools · 12.30K tokens". It is empty when no sub-agent is running.
	Subagent string
}

// Progress reports what the core is doing right now. Cancelling takes
// precedence over a tool call, and a tool call over a running command. It never
// waits on the agent, so a front-end may call it on every frame.
func (c *Chat) Progress() Progress {
	c.mu.Lock()
	defer c.mu.Unlock()

	busy, queued, cancelling := c.inbox.state()
	p := Progress{Queued: queued, Subagent: c.subagent.text()}
	switch {
	case cancelling:
		p.Stage = Cancelling
	case c.pendingTool != "":
		p.Stage, p.Detail, p.Elapsed = RunningTool, c.pendingTool, c.stageElapsed()
	case c.pendingCommand != "":
		p.Stage, p.Detail, p.Elapsed = RunningCommand, c.pendingCommand, c.stageElapsed()
	case busy:
		p.Stage, p.Elapsed = Thinking, c.stageElapsed()
	}
	return p
}

func (c *Chat) stageElapsed() time.Duration {
	if c.stageStart.IsZero() {
		return 0
	}
	return c.now().Sub(c.stageStart)
}
