package command

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/mcp"
)

type recAgent struct {
	model     string
	modelErr  error
	effort    llm.Effort
	models    []string
	compacted int
}

func (a *recAgent) ChangeModel(n string) error  { a.model = n; return a.modelErr }
func (a *recAgent) ChangeEffort(e llm.Effort)   { a.effort = e }
func (a *recAgent) AvailableModels() []string   { return a.models }
func (a *recAgent) ModelInfo() *agent.ModelInfo { return nil }
func (a *recAgent) Compact()                    { a.compacted++ }

type recLine struct {
	kind Kind
	text string
}

type recCtx struct {
	agent    *recAgent
	lines    []recLine
	cleared  int
	clearErr error
}

func newCtx() *recCtx { return &recCtx{agent: &recAgent{}} }

func (c *recCtx) Agent() AgentController { return c.agent }
func (c *recCtx) Print(k Kind, t string) { c.lines = append(c.lines, recLine{k, t}) }
func (c *recCtx) Clear() error           { c.cleared++; return c.clearErr }
func (c *recCtx) last() recLine          { return c.lines[len(c.lines)-1] }

type recMCP struct {
	statuses []mcp.Status
	started  string
	stopped  string
	startErr error
}

func (m *recMCP) GetMCPs() []mcp.Status                        { return m.statuses }
func (m *recMCP) Start(ctx context.Context, name string) error { m.started = name; return m.startErr }
func (m *recMCP) Stop(name string) error                       { m.stopped = name; return nil }

func TestModelSwitches(t *testing.T) {
	c := newCtx()
	Model().Run(c, "gpt-x")
	if c.agent.model != "gpt-x" {
		t.Errorf("ChangeModel not called with gpt-x, got %q", c.agent.model)
	}
	if c.last().kind != Info || !strings.Contains(c.last().text, "gpt-x") {
		t.Errorf("last line = %+v", c.last())
	}
}

func TestModelUsageOnEmpty(t *testing.T) {
	c := newCtx()
	Model().Run(c, "")
	if c.agent.model != "" {
		t.Errorf("ChangeModel should not be called")
	}
	if c.last().kind != Info || !strings.Contains(c.last().text, "Usage") {
		t.Errorf("last line = %+v", c.last())
	}
}

func TestModelReportsError(t *testing.T) {
	c := newCtx()
	c.agent.modelErr = errors.New("no such model")
	Model().Run(c, "bad")
	if c.last().kind != Error || !strings.Contains(c.last().text, "no such model") {
		t.Errorf("last line = %+v", c.last())
	}
}

func TestModelsLists(t *testing.T) {
	c := newCtx()
	c.agent.models = []string{"a", "b"}
	Models().Run(c, "")
	if c.last().kind != Info || !strings.Contains(c.last().text, "a, b") {
		t.Errorf("last line = %+v", c.last())
	}
}

func TestEffortValid(t *testing.T) {
	c := newCtx()
	Effort().Run(c, "medium")
	if c.agent.effort != llm.EffortMedium {
		t.Errorf("effort = %q, want medium", c.agent.effort)
	}
	if c.last().kind != Info {
		t.Errorf("last line = %+v", c.last())
	}
}

func TestEffortInvalid(t *testing.T) {
	c := newCtx()
	Effort().Run(c, "turbo")
	if c.agent.effort != "" {
		t.Errorf("effort should be unchanged, got %q", c.agent.effort)
	}
	if c.last().kind != Error {
		t.Errorf("last line = %+v", c.last())
	}
}

func TestCompactDelegates(t *testing.T) {
	c := newCtx()
	Compact().Run(c, "")
	if c.agent.compacted != 1 {
		t.Errorf("Compact called %d times, want 1", c.agent.compacted)
	}
}

func TestClearSuccess(t *testing.T) {
	c := newCtx()
	Clear().Run(c, "")
	if c.cleared != 1 {
		t.Errorf("Clear called %d times, want 1", c.cleared)
	}
	if c.last().kind != Info || !strings.Contains(c.last().text, "cleared") {
		t.Errorf("last line = %+v", c.last())
	}
}

func TestClearError(t *testing.T) {
	c := newCtx()
	c.clearErr = errors.New("boom")
	Clear().Run(c, "")
	if c.last().kind != Error || !strings.Contains(c.last().text, "boom") {
		t.Errorf("last line = %+v", c.last())
	}
}

func TestMCPListsStatuses(t *testing.T) {
	c := newCtx()
	mgr := &recMCP{statuses: []mcp.Status{{Name: "pw", Active: true}}}
	MCP(mgr).Run(c, "")
	if c.last().kind != Info || !strings.Contains(c.last().text, "pw") || !strings.Contains(c.last().text, "on") {
		t.Errorf("last line = %+v", c.last())
	}
}

func TestMCPStartsNamed(t *testing.T) {
	c := newCtx()
	mgr := &recMCP{statuses: []mcp.Status{{Name: "pw"}}}
	MCP(mgr).Run(c, "on pw")
	if mgr.started != "pw" {
		t.Errorf("Start called with %q, want pw", mgr.started)
	}
}

func TestMCPStartsSoleServerWithoutName(t *testing.T) {
	c := newCtx()
	mgr := &recMCP{statuses: []mcp.Status{{Name: "pw"}}}
	MCP(mgr).Run(c, "on")
	if mgr.started != "pw" {
		t.Errorf("Start called with %q, want pw (sole server)", mgr.started)
	}
}
