package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-chat/theme"
)

type fakeCore struct {
	name   string
	lines  []chat.Line
	busy   bool
	status string
}

func (f *fakeCore) Name() string            { return f.name }
func (f *fakeCore) Theme() theme.Theme      { return theme.Default }
func (f *fakeCore) Transcript() []chat.Line { return f.lines }
func (f *fakeCore) Busy() bool              { return f.busy }
func (f *fakeCore) StatusText() string      { return f.status }
func (f *fakeCore) Submit(string)           {}

func populatedCore() *fakeCore {
	return &fakeCore{
		name:   "CHAT",
		status: "model · ctx:0% · 0 tok",
		lines: []chat.Line{
			{Kind: command.User, Text: "❯ hi"},
			{Kind: command.Reply, Text: "# Hello\n\nworld"},
			{Kind: command.Telemetry, Text: "[5 out tok]"},
			{Kind: command.Activity, Text: "● tool: search"},
			{Kind: command.Error, Text: "Error: boom"},
			{Kind: command.Info, Text: "Context cleared."},
		},
	}
}

func sized(m model, w, h int) model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(model)
}

func TestViewRendersPopulatedCore(t *testing.T) {
	m := sized(newModel(populatedCore()), 80, 24)
	view := m.View()
	if view.Content == "" {
		t.Fatal("View rendered empty content")
	}
	if !strings.Contains(view.Content, "CHAT") {
		t.Errorf("header name missing from view")
	}
}

func TestViewBusyShowsSpinnerStatus(t *testing.T) {
	core := populatedCore()
	core.busy = true
	m := sized(newModel(core), 80, 24)
	if !strings.Contains(m.View().Content, "thinking") {
		t.Errorf("busy view missing thinking status")
	}
}

func TestViewEmptyShowsWelcome(t *testing.T) {
	m := sized(newModel(&fakeCore{name: "CHAT"}), 80, 24)
	content := m.View().Content
	if !strings.Contains(content, "/help") {
		t.Errorf("empty view should hint at /help, got: %q", content)
	}
}

func TestRefreshRebuildsAfterClear(t *testing.T) {
	c := populatedCore()
	m := sized(newModel(c), 80, 24)
	// Simulate /clear: the transcript shrinks to a single line.
	c.lines = []chat.Line{{Kind: command.Info, Text: "Context cleared."}}
	m = sized(m, 80, 24) // re-refresh at the same width
	content := m.View().Content
	if !strings.Contains(content, "Context cleared.") {
		t.Errorf("cleared content missing: %q", content)
	}
	if strings.Contains(content, "boom") {
		t.Errorf("stale pre-clear content still rendered: %q", content)
	}
}

func TestViewBeforeReadyDoesNotPanic(t *testing.T) {
	m := newModel(populatedCore())
	if m.View().Content == "" {
		t.Error("pre-ready view should show a placeholder")
	}
}

func TestEnterSubmitsAndClearsInput(t *testing.T) {
	m := sized(newModel(populatedCore()), 80, 24)
	m.input.SetValue("hello")
	enter, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := enter.(model).input.Value(); got != "" {
		t.Errorf("input not cleared after enter, got %q", got)
	}
}
