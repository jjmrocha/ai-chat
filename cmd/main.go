// Command ai-chat launches the terminal chat UI backed by an ai-toolkit agent.
//
// It connects to OpenRouter, reading the API key from the OPEN_ROUTER_KEY
// environment variable, and registers the Playwright MCP server with the agent's
// tool manager.
//
// Usage:
//
//	export OPEN_ROUTER_KEY=sk-...
//	go run ./cmd
package main

import (
	"context"
	"log"
	"os"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/ui"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/mcp"
	"github.com/jjmrocha/ai-toolkit/tools"
)

func main() {
	client, err := llm.New(llm.Config{
		Provider: llm.ProviderOpenRouter,
		APIKey:   os.Getenv("OPEN_ROUTER_KEY"),
		Model:    "deepseek/deepseek-v4-flash",
		Models: []string{"deepseek/deepseek-v4-flash",
			"deepseek/deepseek-v4-pro",
			"xiaomi/mimo-v2.5"},
		Effort: llm.EffortMedium,
	})
	if err != nil {
		panic(err)
	}

	toolBox := tools.NewToolBox()

	mcpMng := mcp.NewManager(toolBox)
	defer mcpMng.Close()

	mcpMng.Register(mcp.ClientConfig{
		Name:    "playwright",
		Command: "npx",
		Args:    []string{"@playwright/mcp@latest"},
	})

	ag, err := agent.New(agent.Config{}, client, toolBox)
	if err != nil {
		panic(err)
	}

	defer ag.Close()
	ag.StartSession("You are a helpful assistant. You can answer questions and provide information.")

	core := chat.New("CHAT", ag,
		chat.WithMCP(mcpMng),
		chat.WithModelCommand(),
		chat.WithModelsCommand(),
		chat.WithEffortCommand(),
		chat.WithCompactCommand(),
		chat.WithClearCommand(),
		chat.WithThemeCommand(),
	)
	if err := ui.Run(context.Background(), core); err != nil {
		log.Fatal(err)
	}
}
