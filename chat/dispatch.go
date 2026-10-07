package chat

import (
	"context"
	"strings"
	"unicode"
)

// Submit queues text as the next input and returns immediately; the work runs
// on its own goroutine.
//
// Text is trimmed, and empty input is ignored, as is anything submitted after
// [Chat.Close]. Input whose first word is a registered command, such as
// "/model gpt", runs that command; anything else is an agent turn. Input that
// starts with "/" but names no registered command, such as a file path, and
// input for a command registered with [WithSkillCommand] are sent to the agent
// exactly as typed. Commands and turns share one queue and run strictly in
// submission order, so a command never overlaps a turn or another command.
func (c *Chat) Submit(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	ctx, started := c.inbox.push(c.baseCtx, text)
	if started {
		c.work.Add(1)
	}
	c.mu.Unlock()

	c.notify()
	if started {
		go c.process(ctx, text)
	}
}

// Busy reports whether a turn or command is currently running.
func (c *Chat) Busy() bool { return c.inbox.running() }

// Cancel stops the running turn or command and discards all queued input. A
// turn ends with a "Cancelled." line unless its reply had already arrived; a
// command stops once it notices its context was cancelled. Cancel
// does nothing when idle.
func (c *Chat) Cancel() {
	c.inbox.drop()
	c.notify()
}

func (c *Chat) process(ctx context.Context, text string) {
	defer c.work.Done()
	for ok := true; ok; text, ctx, ok = c.inbox.next(c.context()) {
		c.runItem(ctx, text)
	}
}

func (c *Chat) runItem(ctx context.Context, text string) {
	c.agentMu.Lock()
	defer c.agentMu.Unlock()

	defer c.enterWork(ctx)()

	if strings.HasPrefix(text, "/") {
		c.runCommand(ctx, text)
		return
	}
	c.turn(ctx, text)
}

func (c *Chat) runCommand(ctx context.Context, input string) {
	name, args := splitCommand(strings.TrimPrefix(input, "/"))

	cmd, ok := c.commands.Get(name)
	if _, skill := cmd.(skillCommand); !ok || skill {
		c.turn(ctx, input)
		return
	}

	c.setPendingCommand(input)
	defer c.setPendingCommand("")

	cmd.Run(commandContext{c}, strings.TrimSpace(args))
}

func splitCommand(s string) (name, args string) {
	i := strings.IndexFunc(s, unicode.IsSpace)
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i:]
}

func (c *Chat) setPendingCommand(input string) {
	c.mutate(func() { c.pendingCommand = input })
}

func (c *Chat) enterWork(ctx context.Context) (leave func()) {
	c.mu.Lock()
	c.workCtx = ctx
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		c.workCtx = nil
		c.mu.Unlock()
	}
}
