package main

import (
	"context"
	"log"
	"time"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/ui"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/mcp"
	"github.com/jjmrocha/ai-toolkit/packs"
	"github.com/jjmrocha/ai-toolkit/skills"
	"github.com/jjmrocha/ai-toolkit/tools"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	client, err := llm.New(llm.Config{
		Provider: llm.ProviderOllama,
		Model:    "gemma4:31b-cloud",
		Models:   []string{"granite4.2-8b:m1", "gemma4:31b-cloud"},
		Effort:   llm.EffortOff,
	})
	if err != nil {
		return err
	}

	skillColl := skills.NewCollection()

	for _, name := range []string{
		"applying-terseness",
		"grill-me",
		"removing-ai-tells",
		"socratic-mentor",
	} {
		if err := skillColl.AddClaudeSkill(name); err != nil {
			return err
		}
	}

	toolBox := tools.NewToolBox()

	shellPack, err := packs.ShellTools(toolBox)
	if err != nil {
		return err
	}

	defer func() { _ = shellPack.Close() }()

	filePack, err := packs.FileTools(toolBox, ".")
	if err != nil {
		return err
	}

	defer func() { _ = filePack.Close() }()

	mcpMng := mcp.NewManager(toolBox)
	defer mcpMng.Close()

	mcpMng.Register(mcp.ClientConfig{
		Name:            "donsetch",
		Command:         "donsetch",
		Args:            []string{"mcp", "--supervised"},
		ToolCallTimeout: 15 * time.Minute,
	})

	ag, err := agent.New(agent.Config{}, client)
	if err != nil {
		return err
	}

	defer ag.Close()

	ag.StartSession(agent.SessionConfig{
		Prompt:  prompt,
		ToolBox: toolBox,
		Skills:  skillColl,
	})

	core := chat.New("CHAT", ag,
		chat.WithDefaultCommands(),
		chat.WithMCP(mcpMng),
		chat.WithSkills(skillColl),
	)

	return ui.Run(context.Background(), core)
}
