package command

import (
	"context"

	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/mcp"
)

type mockedAgentController struct {
	changeModelFunc     func(name string) error
	changeEffortFunc    func(e llm.Effort) error
	availableModelsFunc func() []string
	compactFunc         func()
}

func (m *mockedAgentController) ChangeModel(name string) error {
	if m.changeModelFunc == nil {
		return nil
	}
	return m.changeModelFunc(name)
}

func (m *mockedAgentController) ChangeEffort(e llm.Effort) error {
	if m.changeEffortFunc == nil {
		return nil
	}
	return m.changeEffortFunc(e)
}

func (m *mockedAgentController) AvailableModels() []string {
	if m.availableModelsFunc == nil {
		return nil
	}
	return m.availableModelsFunc()
}

func (m *mockedAgentController) Compact() {
	if m.compactFunc != nil {
		m.compactFunc()
	}
}

type printedLine struct {
	kind Kind
	text string
}

type mockedContext struct {
	agentFunc func() AgentController
	printFunc func(kind Kind, text string)
	clearFunc func() error
	printed   []printedLine
}

func (m *mockedContext) Agent() AgentController {
	if m.agentFunc == nil {
		return &mockedAgentController{}
	}
	return m.agentFunc()
}

func (m *mockedContext) Print(kind Kind, text string) {
	m.printed = append(m.printed, printedLine{kind: kind, text: text})
	if m.printFunc != nil {
		m.printFunc(kind, text)
	}
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

type mockedSkillsController struct {
	skillsFunc func() []string
}

func (m *mockedSkillsController) Skills() []string {
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
