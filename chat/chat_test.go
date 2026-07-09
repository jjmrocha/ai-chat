package chat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
)

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
func (m *mockAgent) ChangeEffort(e llm.Effort)     { m.changeEffortCalled = e }
func (m *mockAgent) AvailableModels() []string      { return m.availableModels }
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

func TestCommandCompactCallsCompact(t *testing.T) {
	m := &mockAgent{}
	mdl := newModel(m, Config{Name: "test"})
	mdl.busy = false

	mdl, _ = mdl.submit("/compact")

	if !m.compactCalled {
		t.Error("expected CompactContext to be called")
	}
	if len(mdl.transcript) == 0 {
		t.Fatal("expected transcript entries")
	}
	last := mdl.transcript[len(mdl.transcript)-1]
	if !strings.Contains(last, "compacted") {
		t.Errorf("expected compaction block, got %q", last)
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