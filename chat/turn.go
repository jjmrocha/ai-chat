package chat

import (
	"context"

	"github.com/jjmrocha/ai-toolkit/agent"
)

func (c *Chat) turn(ctx context.Context, text string) {
	c.append(User, text)

	resp, err := c.agent.Process(ctx, text)
	c.closeToolCall(formatToolResult("", nil, 0))

	c.recordTurn(resp, err)
	c.reportTurn(ctx, resp, err)
}

func (c *Chat) recordTurn(resp *agent.Response, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil && resp != nil {
		c.lastMeta = resp.Metadata
	}
	if c.model.info == nil {
		c.model.fresh = false
	}
}

func (c *Chat) reportTurn(ctx context.Context, resp *agent.Response, err error) {
	switch {
	case err != nil && cancelledByUser(ctx):
		c.append(Info, "Cancelled.")
	case err != nil:
		c.append(Error, "Error: "+err.Error())
	case resp != nil:
		c.append(Reply, resp.Content)
		if line := c.telemetryFmt(resp.Metadata); line != "" {
			c.append(Telemetry, line)
		}
	default:
		c.append(Error, "No response received.")
	}
}
