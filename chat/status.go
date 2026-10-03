package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
)

// StatusFormatter renders the status line a front-end shows below the input.
// Install one with [WithStatusFormatter].
type StatusFormatter func(StatusInfo) string

type modelState struct {
	info     *agent.ModelInfo
	fresh    bool
	cancel   context.CancelFunc
	reported bool
	gen      uint64
}

// Status returns the current model, effort and context usage. It never waits
// on the agent, so a front-end may call it on every frame: the model is looked
// up in the background, once per model or effort change, and the observer is
// notified when the answer lands. Until then the model fields are zero. A
// failed lookup is retried after the next turn.
func (c *Chat) Status() StatusInfo {
	c.mu.Lock()
	info := c.statusLocked()
	lookup := c.startLookupLocked()
	c.mu.Unlock()

	if lookup != nil {
		go lookup()
	}
	return info
}

// StatusText returns [Chat.Status] rendered by the status formatter.
func (c *Chat) StatusText() string { return c.statusFmt(c.Status()) }

func (c *Chat) statusLocked() StatusInfo {
	info := StatusInfo{Tokens: c.lastMeta.TotalTokens}
	mi := c.model.info
	if mi == nil {
		return info
	}

	info.Name = sanitize(mi.ModelName)
	info.Provider = mi.Provider
	info.Effort = mi.Effort
	if mi.ModelContextSize > 0 {
		info.CtxPct = float64(info.Tokens) * 100 / float64(mi.ModelContextSize)
	}
	return info
}

func (c *Chat) startLookupLocked() func() {
	if c.model.fresh || c.model.cancel != nil || c.closed {
		return nil
	}

	ctx, cancel := context.WithCancel(c.baseCtx)
	c.model.cancel = cancel
	gen := c.model.gen
	c.work.Add(1)
	return func() {
		defer c.work.Done()
		defer cancel()
		c.lookupModel(ctx, gen)
	}
}

func (c *Chat) lookupModel(ctx context.Context, gen uint64) {
	c.agentMu.Lock()
	info := c.agent.ModelInfo(ctx)
	c.agentMu.Unlock()

	c.mutate(func() {
		c.model.cancel = nil
		if gen != c.model.gen {
			return
		}
		c.model.info = info
		c.model.fresh = true
		if info != nil {
			c.model.reported = false
		}
	})
}

func (c *Chat) invalidateModel() {
	c.mu.Lock()
	c.model.fresh = false
	c.model.reported = false
	c.model.gen++
	c.mu.Unlock()
}

func defaultStatusFormatter(info StatusInfo) string {
	name := info.Name
	if name == "" {
		name = "—"
	}
	if info.Provider != "" {
		name = fmt.Sprintf("%s (%s)", name, info.Provider)
	}
	parts := []string{name}
	if info.Effort != llm.EffortOff && info.Effort != "" {
		parts = append(parts, string(info.Effort))
	}
	parts = append(parts, fmt.Sprintf("ctx: %.0f%%", info.CtxPct))
	parts = append(parts, fmt.Sprintf("tokens: %s", formatTokens(info.Tokens)))
	return strings.Join(parts, " · ")
}
