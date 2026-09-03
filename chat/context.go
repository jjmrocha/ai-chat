package chat

import (
	"fmt"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-chat/theme"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
)

var (
	_ command.Context         = (*Chat)(nil)
	_ command.AgentController = (*Chat)(nil)
)

// Print implements command.Context.
func (c *Chat) Print(kind command.Kind, text string) { c.append(kind, text) }

// Agent implements command.Context, exposing the agent operations commands may
// drive. The core is its own controller, supplying the request context.
func (c *Chat) Agent() command.AgentController { return c }

// Clear implements command.Context: reset the agent session and empty the
// transcript. On reset failure the transcript is left intact.
func (c *Chat) Clear() error {
	if err := c.agent.ResetSession(); err != nil {
		return err
	}
	c.mu.Lock()
	c.transcript = nil
	c.mu.Unlock()
	c.notify()
	return nil
}

// ChangeTheme implements command.Context: switch the active color palette.
func (c *Chat) ChangeTheme(name string) error {
	t, ok := theme.ByName(name)
	if !ok {
		return fmt.Errorf("unknown theme %q", name)
	}
	c.mu.Lock()
	c.theme = t
	c.mu.Unlock()
	c.notify()
	return nil
}

// ChangeModel implements command.AgentController.
func (c *Chat) ChangeModel(name string) error { return c.agent.ChangeModel(name) }

// ChangeEffort implements command.AgentController.
func (c *Chat) ChangeEffort(e llm.Effort) error { return c.agent.ChangeEffort(e) }

// AvailableModels implements command.AgentController.
func (c *Chat) AvailableModels() []string { return c.agent.AvailableModels() }

// ModelInfo implements command.AgentController.
func (c *Chat) ModelInfo() *agent.ModelInfo { return c.agent.ModelInfo(c.baseCtx) }

// Compact implements command.AgentController: run context compaction. The
// outcome arrives through the agent's feedback events.
func (c *Chat) Compact() { c.agent.CompactContext(c.baseCtx) }
