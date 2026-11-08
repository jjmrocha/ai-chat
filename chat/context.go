package chat

import (
	"context"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/agent"
)

var (
	_ command.Context         = commandContext{}
	_ command.AgentController = commandContext{}
	_ command.Quitter         = commandContext{}
)

type commandContext struct{ c *Chat }

func (cc commandContext) Info(text string) { cc.c.append(Info, text) }

func (cc commandContext) Error(text string) { cc.c.append(Error, text) }

func (cc commandContext) Agent() command.AgentController { return cc }

func (cc commandContext) Clear() error { return cc.c.clear() }

func (cc commandContext) Context() context.Context { return cc.c.workContext() }

func (cc commandContext) Quit() { cc.c.quit() }

func (cc commandContext) Compact() {
	ctx := cc.c.workContext()
	cc.c.agent.CompactContext(ctx)
	if cancelledByUser(ctx) {
		cc.c.append(Info, "Cancelled.")
	}
}

func (c *Chat) clear() error {
	if err := c.agent.ResetSession(); err != nil {
		return err
	}
	c.mutate(func() {
		c.transcript = nil
		c.epoch++
		c.lastMeta = agent.Metadata{}
	})
	return nil
}

func (c *Chat) workContext() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.workCtx != nil {
		return c.workCtx
	}
	return c.baseCtx
}
