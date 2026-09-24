package chat

import (
	"context"
	"strings"

	"github.com/jjmrocha/ai-chat/command"
)

// Submit queues text as the next input and returns immediately; the work runs
// on its own goroutine.
//
// Text is trimmed, and empty input is ignored, as is anything submitted after
// [Chat.Close]. Input beginning with "/" runs as a slash command, anything else
// as an agent turn. Commands and turns share one queue and run strictly in
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

// Queued reports whether input submitted during a running turn is still
// waiting to run.
func (c *Chat) Queued() bool { return c.inbox.waiting() }

// Cancel stops the running turn or command and discards all queued input. A
// turn ends with a "Cancelled." line unless its reply had already arrived; a
// command stops once it notices its [command.Context] was cancelled. Cancel
// does nothing when idle.
func (c *Chat) Cancel() {
	c.inbox.drop()
	c.notify()
}

// Cancelling reports whether [Chat.Cancel] was called and the running turn or
// command has not yet returned. A front-end can show it in place of progress
// detail.
func (c *Chat) Cancelling() bool { return c.inbox.cancelPending() }

// PendingCommand returns the slash command in flight, as it was typed, or an
// empty string when no command is running. A front-end can show it as progress
// detail, to distinguish waiting on a command from waiting on the model.
func (c *Chat) PendingCommand() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pendingCommand
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
		c.runCommand(text)
		return
	}
	c.turn(ctx, text)
}

func (c *Chat) runCommand(input string) {
	name, args, _ := strings.Cut(strings.TrimPrefix(input, "/"), " ")

	cmd, ok := c.commands.Get(name)
	if !ok {
		c.append(command.Error, "Error: unknown command /"+name)
		return
	}

	c.setPendingCommand(input)
	defer c.setPendingCommand("")

	cmd.Run(c, strings.TrimSpace(args))
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
