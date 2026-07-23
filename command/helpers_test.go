package command

import (
	"context"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/mcp"
)

type mockedAgentController struct {
	changeModelFunc     func(name string) error
	changeEffortFunc    func(e llm.Effort)
	availableModelsFunc func() []string
	modelInfoFunc       func() *agent.ModelInfo
	compactFunc         func()
}

func (m *mockedAgentController) ChangeModel(name string) error {
	if m.changeModelFunc == nil {
		return nil
	}
	return m.changeModelFunc(name)
}

func (m *mockedAgentController) ChangeEffort(e llm.Effort) {
	if m.changeEffortFunc != nil {
		m.changeEffortFunc(e)
	}
}

func (m *mockedAgentController) AvailableModels() []string {
	if m.availableModelsFunc == nil {
		return nil
	}
	return m.availableModelsFunc()
}

func (m *mockedAgentController) ModelInfo() *agent.ModelInfo {
	if m.modelInfoFunc == nil {
		return nil
	}
	return m.modelInfoFunc()
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
	agentFunc       func() AgentController
	printFunc       func(kind Kind, text string)
	clearFunc       func() error
	changeThemeFunc func(name string) error
	printed         []printedLine
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

func (m *mockedContext) ChangeTheme(name string) error {
	if m.changeThemeFunc == nil {
		return nil
	}
	return m.changeThemeFunc(name)
}

type mockedMCPController struct {
	getMCPsFunc func() []mcp.Status
	startFunc   func(ctx context.Context, name string) error
	stopFunc    func(name string) error
}

func (m *mockedMCPController) GetStatus() []mcp.Status {
	if m.getMCPsFunc == nil {
		return nil
	}
	return m.getMCPsFunc()
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
