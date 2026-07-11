package chat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/mcp"
)

type mockMCP struct {
	statuses []mcp.Status
	started  []string
	stopped  []string
	startErr error
	stopErr  error
}

func (m *mockMCP) GetMCPs() []mcp.Status { return m.statuses }
func (m *mockMCP) Start(ctx context.Context, name string) error {
	m.started = append(m.started, name)
	return m.startErr
}
func (m *mockMCP) Stop(name string) error {
	m.stopped = append(m.stopped, name)
	return m.stopErr
}

type mockAgent struct {
	changeModelCalled  string
	changeEffortCalled llm.Effort
	availableModels    []string
	compactCalled      bool
	changeModelErr     error
	modelInfo          *agent.ModelInfo
}

func (m *mockAgent) Process(ctx context.Context, input string) (*agent.Response, error) {
	return nil, nil
}
func (m *mockAgent) ResetSession() error { return nil }
func (m *mockAgent) ModelInfo(ctx context.Context) *agent.ModelInfo {
	if m.modelInfo != nil {
		return m.modelInfo
	}
	return &agent.ModelInfo{
		ModelName:        "test",
		ModelContextSize: 128000,
		Effort:           llm.EffortOff,
	}
}
func (m *mockAgent) ChangeModel(model string) error {
	m.changeModelCalled = model
	return m.changeModelErr
}
func (m *mockAgent) ChangeEffort(e llm.Effort)          { m.changeEffortCalled = e }
func (m *mockAgent) AvailableModels() []string          { return m.availableModels }
func (m *mockAgent) CompactContext(ctx context.Context) { m.compactCalled = true }

func TestCommandModelDispatchesChangeModel(t *testing.T) {
	m := &mockAgent{}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = false

	mdl, _ = mdl.submit("/model gpt-4")

	if m.changeModelCalled != "gpt-4" {
		t.Errorf("expected ChangeModel(gpt-4), got %q", m.changeModelCalled)
	}
	if len(mdl.transcript) == 0 {
		t.Fatal("expected transcript entries")
	}
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "Switched to: gpt-4") {
		t.Errorf("expected success block, got %q", last)
	}
}

func TestCommandModelWithErrorShowsInfoBlock(t *testing.T) {
	m := &mockAgent{changeModelErr: errors.New("model not found")}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = false

	mdl, _ = mdl.submit("/model unknown")
	if len(mdl.transcript) == 0 {
		t.Fatal("expected transcript entries")
	}
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "model not found") {
		t.Errorf("expected error block containing 'model not found', got %q", last)
	}
}

func TestCommandModelWithNoNameShowsUsage(t *testing.T) {
	m := &mockAgent{}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = false

	mdl, _ = mdl.submit("/model")
	if len(mdl.transcript) == 0 {
		t.Fatal("expected transcript entries")
	}
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "Usage") {
		t.Errorf("expected usage block, got %q", last)
	}
}

func TestCommandModelsListsAvailable(t *testing.T) {
	m := &mockAgent{availableModels: []string{"gpt-4", "claude-3"}}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = false

	mdl, _ = mdl.submit("/models")
	if len(mdl.transcript) == 0 {
		t.Fatal("expected transcript entries")
	}
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "gpt-4") || !strings.Contains(last, "claude-3") {
		t.Errorf("expected model list, got %q", last)
	}
}

func TestCommandModelsWithEmptyListShowsMessage(t *testing.T) {
	m := &mockAgent{}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = false

	mdl, _ = mdl.submit("/models")
	if len(mdl.transcript) == 0 {
		t.Fatal("expected transcript entries")
	}
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "No model list") {
		t.Errorf("expected 'no model list', got %q", last)
	}
}

func TestCommandEffortDispatchesChangeEffort(t *testing.T) {
	m := &mockAgent{}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = false

	mdl, _ = mdl.submit("/effort medium")

	if m.changeEffortCalled != llm.EffortMedium {
		t.Errorf("expected ChangeEffort(medium), got %q", m.changeEffortCalled)
	}
	if len(mdl.transcript) == 0 {
		t.Fatal("expected transcript entries")
	}
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "medium") {
		t.Errorf("expected success block, got %q", last)
	}
}

func TestCommandEffortInvalidShowsError(t *testing.T) {
	m := &mockAgent{}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = false

	mdl, _ = mdl.submit("/effort extreme")
	if len(mdl.transcript) == 0 {
		t.Fatal("expected transcript entries")
	}
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "Effort must be") {
		t.Errorf("expected error for invalid effort, got %q", last)
	}
}

func TestCommandCompactRunsCompactionAsync(t *testing.T) {
	m := &mockAgent{}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = false

	mdl, cmd := mdl.submit("/compact")

	if cmd == nil {
		t.Fatal("expected a command to run compaction")
	}
	if m.compactCalled {
		t.Error("compaction must run in the command, not on the UI goroutine")
	}
	cmd() // execute the async compaction
	if !m.compactCalled {
		t.Error("expected CompactContext to be called")
	}
	if len(mdl.transcript) != 0 {
		t.Errorf("expected no synchronous transcript entry, got %v", mdl.transcript)
	}
}

func TestCompactFeedbackMessagesRender(t *testing.T) {
	mdl := newModel(&mockAgent{}, Config{Name: "test"})

	next, _ := mdl.Update(compactedMsg{})
	ok := next.(model)
	if len(ok.transcript) == 0 || !strings.Contains(ok.transcript[len(ok.transcript)-1], "compacted") {
		t.Errorf("expected compacted message, got %v", ok.transcript)
	}

	next, _ = mdl.Update(compactFailedMsg{})
	failed := next.(model)
	if len(failed.transcript) == 0 || !strings.Contains(failed.transcript[len(failed.transcript)-1], "compaction failed") {
		t.Errorf("expected failure message, got %v", failed.transcript)
	}
}

func TestCommandMCPWithoutControllerShowsMessage(t *testing.T) {
	mdl := newModel(&mockAgent{}, Config{Name: "test"})
	mdl.busy = false

	mdl, _ = mdl.submit("/mcp")
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "No MCP servers configured") {
		t.Errorf("expected 'no MCP configured', got %q", last)
	}
}

func TestCommandMCPListsStatus(t *testing.T) {
	mc := &mockMCP{statuses: []mcp.Status{{Name: "playwright", Active: true}, {Name: "fs", Active: false}}}
	mdl := newModel(&mockAgent{}, Config{Name: "test", MCP: mc})
	mdl.busy = false

	mdl, _ = mdl.submit("/mcp")
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "playwright: on") || !strings.Contains(last, "fs: off") {
		t.Errorf("expected status listing, got %q", last)
	}
}

func TestCommandMCPOnResolvesSoleServer(t *testing.T) {
	mc := &mockMCP{statuses: []mcp.Status{{Name: "playwright"}}}
	mdl := newModel(&mockAgent{}, Config{Name: "test", MCP: mc})
	mdl.busy = false

	_, cmd := mdl.submit("/mcp on")
	if cmd == nil {
		t.Fatal("expected a command")
	}
	msg := cmd()
	if len(mc.started) != 1 || mc.started[0] != "playwright" {
		t.Errorf("expected Start(playwright), got %v", mc.started)
	}
	done, ok := msg.(mcpDoneMsg)
	if !ok || done.action != "on" || done.name != "playwright" || done.err != nil {
		t.Errorf("unexpected msg: %#v", msg)
	}
}

func TestCommandMCPOffDispatchesStop(t *testing.T) {
	mc := &mockMCP{statuses: []mcp.Status{{Name: "playwright", Active: true}}}
	mdl := newModel(&mockAgent{}, Config{Name: "test", MCP: mc})
	mdl.busy = false

	_, cmd := mdl.submit("/mcp off playwright")
	if cmd == nil {
		t.Fatal("expected a command")
	}
	cmd()
	if len(mc.stopped) != 1 || mc.stopped[0] != "playwright" {
		t.Errorf("expected Stop(playwright), got %v", mc.stopped)
	}
}

func TestCommandMCPOnWithoutNameAndMultipleErrors(t *testing.T) {
	mc := &mockMCP{statuses: []mcp.Status{{Name: "a"}, {Name: "b"}}}
	mdl := newModel(&mockAgent{}, Config{Name: "test", MCP: mc})
	mdl.busy = false

	mdl, cmd := mdl.submit("/mcp on")
	if cmd != nil {
		t.Error("expected no command when the target is ambiguous")
	}
	if len(mc.started) != 0 {
		t.Error("expected no Start call")
	}
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "Specify an MCP name") {
		t.Errorf("expected name-required error, got %q", last)
	}
}

func TestMCPDoneMessageRendersError(t *testing.T) {
	mdl := newModel(&mockAgent{}, Config{Name: "test"})

	next, _ := mdl.Update(mcpDoneMsg{action: "on", name: "playwright", err: errors.New("boom")})
	m := next.(model)
	last := m.transcript[len(m.transcript)-1]
	if !strings.Contains(last, "boom") {
		t.Errorf("expected error render, got %q", last)
	}
}

func TestStatusLineShowsProvider(t *testing.T) {
	m := &mockAgent{modelInfo: &agent.ModelInfo{
		Provider:         llm.ProviderOpenRouter,
		ModelName:        "deepseek/deepseek-v4-flash",
		ModelContextSize: 128000,
		Effort:           llm.EffortOff,
	}}
	mdl := newModel(m, Config{Name: "test"})

	line := mdl.statusLine()
	if !strings.Contains(line, "deepseek/deepseek-v4-flash (openrouter)") {
		t.Errorf("expected model name with provider in parens, got %q", line)
	}
}

func TestCommandsIgnoredWhenBusy(t *testing.T) {
	m := &mockAgent{}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = true
	initialLen := len(mdl.transcript)

	mdl, _ = mdl.submit("/model gpt-4")
	mdl, _ = mdl.submit("/effort medium")
	mdl, _ = mdl.submit("/compact")

	if m.changeModelCalled != "" {
		t.Error("expected no ChangeModel call when busy")
	}
	if m.changeEffortCalled != "" {
		t.Error("expected no ChangeEffort call when busy")
	}
	if m.compactCalled {
		t.Error("expected no CompactContext call when busy")
	}
	if len(mdl.transcript) != initialLen {
		t.Errorf("expected no transcript changes, added %d entries", len(mdl.transcript)-initialLen)
	}
}

func TestUnknownCommandShowsError(t *testing.T) {
	m := &mockAgent{}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = false

	mdl, _ = mdl.submit("/bogus")
	if len(mdl.transcript) == 0 {
		t.Fatal("expected transcript entries")
	}
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "unknown command") {
		t.Errorf("expected error for unknown command, got %q", last)
	}
}

func TestCommandThemeWithoutArgListsThemes(t *testing.T) {
	m := &mockAgent{}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = false

	mdl, _ = mdl.submit("/theme")
	if len(mdl.transcript) == 0 {
		t.Fatal("expected transcript entries")
	}
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "default") || !strings.Contains(last, "nord") {
		t.Errorf("expected theme list, got %q", last)
	}
}

func TestCommandThemeSwitchesTheme(t *testing.T) {
	m := &mockAgent{}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = false

	mdl, _ = mdl.submit("/theme nord")
	if len(mdl.transcript) == 0 {
		t.Fatal("expected transcript entries")
	}
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "nord") {
		t.Errorf("expected success message for nord, got %q", last)
	}
}

func TestCommandThemeInvalidShowsError(t *testing.T) {
	m := &mockAgent{}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = false

	mdl, _ = mdl.submit("/theme bogus")
	if len(mdl.transcript) == 0 {
		t.Fatal("expected transcript entries")
	}
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "Unknown theme") {
		t.Errorf("expected error for unknown theme, got %q", last)
	}
}

func TestDefaultThemeAppliedWhenConfigEmpty(t *testing.T) {
	mdl := newModel(&mockAgent{}, Config{Name: "test"})
	if mdl.cfg.ThemeName != "" {
		t.Fatal("expected empty theme name")
	}
	if len(mdl.transcript) != 0 {
		t.Error("expected empty transcript after init")
	}
}

func TestInvalidThemeNameFallsBackToDefault(t *testing.T) {
	mdl := newModel(&mockAgent{}, Config{Name: "test", ThemeName: "nonexistent"})
	if len(mdl.transcript) != 0 {
		t.Error("expected empty transcript after init with invalid theme")
	}
}
