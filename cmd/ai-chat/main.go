// Command ai-chat launches the terminal chat UI backed by an ai-toolkit agent.
//
// It is the generic showcase: a local Ollama model, no API key, and a spread of
// the toolkit's tool sources. Shell and file tools are served in-process and are
// always on; the web tools run behind an MCP server that stays stopped until
// /mcp on donsetch, so their schema costs nothing until asked for.
//
// Usage:
//
//	go run ./cmd/ai-chat
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
		Model:    "granite4.2-8b:m1",
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

	shellPack := packs.ShellTools(toolBox)
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
