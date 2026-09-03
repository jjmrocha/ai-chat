// Command joe launches a coding agent backed by an ai-toolkit agent.
//
// It connects to OpenRouter, reading the API key from the OPEN_ROUTER_KEY
// environment variable, and takes its whole toolset from the coding pack, which
// is served by Serena. Needs the uvx executable on PATH.
//
// Usage:
//
//	export OPEN_ROUTER_KEY=sk-...
//	go run ./cmd/joe
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
			"deepseek/deepseek-v4-pro",
		},
		Effort: llm.EffortMedium,
	})
	if err != nil {
		return err
	}

	skillColl := skills.NewCollection()

	err = cmdutil.AddSkills(skillColl,
		"analyze-code",
		"brainstorm",
		"coding-discipline",
		"designing-interfaces",
		"guiding-manual-testing",
		"knowledge-base",
		"research",
		"style-checker",
		"test-driven-development",
		"using-software-specialists",
		"writing-unit-tests",
	)
	if err != nil {
		return err
	}

	toolBox := tools.NewToolBox()
	ctx := context.Background()

	codePack, err := packs.CodingTools(ctx, toolBox)
	if err != nil {
		return err
	}

	defer func() { _ = codePack.Close() }()

	if err := toolBox.Add(repoInfoTool, repoInfo); err != nil {
		return err
	}

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

	core := chat.New("JOE", ag,
		chat.WithDefaultCommands(),
		chat.WithSkills(skillColl),
	)

	return ui.Run(ctx, core)
}
