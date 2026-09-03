// Command warren launches a financial analyst backed by an ai-toolkit agent.
//
// It connects to OpenRouter, reading the API key from the OPEN_ROUTER_KEY
// environment variable. Web tools and file tools are always on; market data
// comes from the yfinance MCP server, which is stopped until /mcp on
// yfinance-mcp. Needs the uvx executable on PATH.
//
// Usage:
//
//	export OPEN_ROUTER_KEY=sk-...
//	go run ./cmd/warren
package main

import (
	"context"
	"log"
	"os"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/cmd/internal"
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
		Provider: llm.ProviderOpenRouter,
		APIKey:   os.Getenv("OPEN_ROUTER_KEY"),
		Model:    "z-ai/glm-5.3-flash",
		Models: []string{
			"z-ai/glm-5.3-flash",
			"deepseek/deepseek-v4-flash",
		},
		Effort: llm.EffortMedium,
	})
	if err != nil {
		return err
	}

	skillColl := skills.NewCollection()

	err = cmdutil.AddSkills(skillColl,
		"buffett-valuation",
		"company-research",
	)
	if err != nil {
		return err
	}

	toolBox := tools.NewToolBox()

	filePack, err := packs.FileTools(toolBox, ".")
	if err != nil {
		return err
	}

	defer func() { _ = filePack.Close() }()

	ctx := context.Background()

	webPack, err := packs.WebTools(ctx, toolBox)
	if err != nil {
		return err
	}

	defer func() { _ = webPack.Close() }()

	mcpMng := mcp.NewManager(toolBox)
	defer mcpMng.Close()

	mcpMng.Register(mcp.ClientConfig{
		Name:    "yfinance-mcp",
		Command: "uvx",
		Args:    []string{"yfmcp@latest"},
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

	core := chat.New("WARREN", ag,
		chat.WithDefaultCommands(),
		chat.WithMCP(mcpMng),
		chat.WithSkills(skillColl),
	)

	return ui.Run(ctx, core)
}
