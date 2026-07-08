// Command chatdemo launches the chat UI against a single model so the terminal
// interface can be tried by hand. It registers no tools.
//
// Usage:
//
//	# Ollama (no API key needed), default provider:
//	go run ./cmd/chatdemo -model llama3.2
//
//	# OpenRouter (reads OPENROUTER_API_KEY from the environment):
//	export OPENROUTER_API_KEY=sk-...
//	go run ./cmd/chatdemo -provider openrouter -model openai/gpt-4o-mini
package main

import (
	"context"
	"log"
	"os"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/mcp"
	"github.com/jjmrocha/ai-toolkit/tools"
)

func main() {
	client, err := llm.New(llm.Config{
		Provider: llm.ProviderOpenRouter,
		APIKey:   os.Getenv("OPEN_ROUTER_KEY"),
		Model:    "minimax/minimax-m2.7",
		Effort:   llm.EffortMedium,
	})
	if err != nil {
		panic(err)
	}

	toolBox := tools.NewToolBox()

	mcpClient, err := mcp.NewClient(context.Background(), mcp.ClientConfig{
		Name:    "playwright",
		Command: "npx",
		Args:    []string{"@playwright/mcp@latest"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer mcpClient.Close()

	err = mcpClient.RegisterTools(context.Background(), toolBox)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.New(agent.Config{}, client, toolBox)
	if err != nil {
		panic(err)
	}

	defer ag.Close()
	ag.StartSession("You are a helpful assistant. You can answer questions and provide information.")

	c := chat.New(ag, chat.Config{
		Name:        "CHAT",
		Description: "A minimal terminal chat over an LLM agent.",
	})
	c.Run(context.Background())
}
