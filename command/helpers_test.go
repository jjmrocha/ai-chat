package command

import (
	"context"

	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/mcp"
)

type mockedModelSwitcher struct {
	availableModelsFunc func() []string
	changeModelFunc     func(ctx context.Context, name string) (*llm.ModelInfo, error)
}

func (m *mockedModelSwitcher) AvailableModels() []string {
	if m.availableModelsFunc == nil {
		return nil
	}
	return m.availableModelsFunc()
}

func (m *mockedModelSwitcher) ChangeModel(ctx context.Context, name string) (*llm.ModelInfo, error) {
	if m.changeModelFunc == nil {
		return &llm.ModelInfo{Name: name}, nil
	}
	return m.changeModelFunc(ctx, name)
}

type mockedEffortSwitcher struct {
	changeEffortFunc func(e llm.Effort) error
}

func (m *mockedEffortSwitcher) ChangeEffort(e llm.Effort) error {
	if m.changeEffortFunc == nil {
		return nil
	}
	return m.changeEffortFunc(e)
}

type stream int

const (
	infoStream stream = iota
	errorStream
)

type printedLine struct {
	kind stream
	text string
}

type mockedContext struct {
	ctx         context.Context
	clearFunc   func() error
	compactFunc func()
	printed     []printedLine
}

func (m *mockedContext) Compact() {
	if m.compactFunc != nil {
		m.compactFunc()
	}
}

func (m *mockedContext) Info(text string) {
	m.printed = append(m.printed, printedLine{kind: infoStream, text: text})
}

func (m *mockedContext) Error(text string) {
	m.printed = append(m.printed, printedLine{kind: errorStream, text: text})
}

func (m *mockedContext) Context() context.Context {
	if m.ctx == nil {
		return context.Background()
	}
	return m.ctx
}

func (m *mockedContext) Clear() error {
	if m.clearFunc == nil {
		return nil
	}
	return m.clearFunc()
}

type mockedMCPController struct {
	statusFunc func() []mcp.Status
	startFunc  func(ctx context.Context, name string) error
	stopFunc   func(name string) error
}

func (m *mockedMCPController) Status() []mcp.Status {
	if m.statusFunc == nil {
		return nil
	}
	return m.statusFunc()
}

func (m *mockedMCPController) Start(ctx context.Context, name string) error {
	if m.startFunc == nil {
		return nil
	}
	return m.startFunc(ctx, name)
}

func (m *mockedMCPController) Stop(name string) error {
	if m.stopFunc == nil {
		return nil
	}
	return m.stopFunc(name)
}

type mockedSkillCatalog struct {
	skillsFunc func() []string
}

func (m *mockedSkillCatalog) Skills() []string {
	if m.skillsFunc == nil {
		return nil
	}
	return m.skillsFunc()
}

type mockedQuitter struct {
	quits int
}

func (m *mockedQuitter) Quit() { m.quits++ }

type mockedRegistry struct {
	commandsFunc func() []Command
}

func (m *mockedRegistry) Commands() []Command {
	if m.commandsFunc == nil {
		return nil
	}
	return m.commandsFunc()
}

type stubCommand struct {
	name string
	help string
}

func (s stubCommand) Name() string        { return s.name }
func (s stubCommand) Help() string        { return s.help }
func (s stubCommand) Run(Context, string) {}

type argumentedStub struct {
	stubCommand
	args string
}

func (a argumentedStub) Args() string { return a.args }
