# ai-chat

Build terminal chat agents in Go. Bring an [ai-toolkit](https://github.com/jjmrocha/ai-toolkit) agent; get a headless, testable chat core, a Bubble Tea TUI, a pluggable slash-command framework, themes, and MCP server management.

[![Go Reference](https://pkg.go.dev/badge/github.com/jjmrocha/ai-chat.svg)](https://pkg.go.dev/github.com/jjmrocha/ai-chat)
[![Go 1.26+](https://img.shields.io/badge/go-1.26+-00ADD8)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

## Why ai-chat

- **Headless core, not a monolith.** `chat.Chat` runs the transcript and drives the agent with no terminal attached — drive it from a test, a different UI, or the bundled TUI.
- **Slash commands are pluggable.** Ship the built-ins (`/model`, `/effort`, `/mcp`, `/theme`, …) or implement `command.Command` and register your own. No forking required.
- **Swappable renderer.** The `ui` package is one consumer of the core, wired through a single `Observer` interface. Replace it without touching your agent logic.
- **MCP built in.** Register MCP servers and toggle them at runtime with `/mcp`.

## Install

```bash
go get github.com/jjmrocha/ai-chat
```

Requires Go 1.26+.

## Quickstart

A complete terminal chat agent backed by OpenRouter. Drop this into `main.go` in your own module, set your key, and run it with `go run .`.

```bash
export OPEN_ROUTER_KEY=sk-...
go run .
```

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/ui"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/tools"
)

func main() {
	client, err := llm.New(llm.Config{
		Provider: llm.ProviderOpenRouter,
		APIKey:   os.Getenv("OPEN_ROUTER_KEY"),
		Model:    "deepseek/deepseek-v4-flash",
		Models:   []string{"deepseek/deepseek-v4-flash", "deepseek/deepseek-v4-pro"},
		Effort:   llm.EffortMedium,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.New(agent.Config{}, client, tools.NewToolBox())
	if err != nil {
		log.Fatal(err)
	}
	defer ag.Close()
	ag.StartSession("You are a helpful assistant.")

	core := chat.New("CHAT", ag,
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
```

A fuller example wiring the Playwright MCP server lives in [`cmd/main.go`](cmd/main.go).

## Architecture

```
        ui (Bubble Tea)          ← swappable renderer
          │  Observer
          ▼
     chat.Chat (headless core)   ← transcript + command dispatch
        │            │
   command registry  ai-toolkit agent
   (/model, /mcp,     │      │
    your own…)        llm    mcp / tools
```

`ui` is just one `Observer`. The core notifies it via `TranscriptChanged()` / `Quit()`; everything the UI needs it reads back through `chat.Chat`'s methods. Swap `ui` for your own front-end without changing the core.

## Recipes

### Add a custom slash command

Implement `command.Command` and register it with `chat.WithCommand`.

```go
type pingCmd struct{}

func (pingCmd) Name() string { return "ping" }
func (pingCmd) Help() string { return "/ping           Reply with pong" }
func (pingCmd) Run(ctx command.Context, args string) {
	ctx.Print(command.Info, "pong "+args)
}

core := chat.New("CHAT", ag, chat.WithCommand(pingCmd{}))
```

`command.Context` gives a command access to the agent (`Agent()`), the transcript (`Print`), session reset (`Clear`), and theme switching (`ChangeTheme`).

### Register an MCP server

```go
toolBox := tools.NewToolBox()
mcpMng := mcp.NewManager(toolBox)
defer mcpMng.Close()

mcpMng.Register(mcp.ClientConfig{
	Name:    "playwright",
	Command: "npx",
	Args:    []string{"@playwright/mcp@latest"},
})

ag, _ := agent.New(agent.Config{}, client, toolBox)
core := chat.New("CHAT", ag, chat.WithMCP(mcpMng))
```

Users then toggle servers at runtime: `/mcp on playwright`, `/mcp off playwright`, `/mcp` to list.

> **Security:** `ClientConfig.Command` and `Args` are executed with `os/exec` **without a shell**, so they are trusted input. Populate them from operator configuration, never from untrusted user input.

### Add a theme

```go
core := chat.New("CHAT", ag, chat.WithTheme(theme.Nord))
```

Built-ins: `theme.Default`, `theme.Nord`, `theme.Monokai`, `theme.Catppuccin`. Users switch at runtime with `/theme <name>`; `theme.Names()` lists them.

### Drive the core headless (for tests)

No terminal required — the core is a plain object you can submit input to and read back.

```go
core := chat.New("CHAT", ag)
core.Submit("hello")
for _, line := range core.Transcript() {
	fmt.Println(line.Text)
}
```

## Built-in commands

| Constructor | Command | Effect |
|---|---|---|
| `WithModelCommand()` | `/model <name>` | Switch active model |
| `WithModelsCommand()` | `/models` | List available models |
| `WithEffortCommand()` | `/effort <level>` | Set reasoning effort (`off`, `low`, `medium`, `max`) |
| `WithClearCommand()` | `/clear` | Reset conversation |
| `WithCompactCommand()` | `/compact` | Force context compaction |
| `WithThemeCommand()` | `/theme [name]` | Show or switch theme |
| `WithMCP(mgr)` | `/mcp [on\|off] [name]` | Show or toggle MCP servers |

## Packages

| Package | Purpose |
|---|---|
| `chat` | Headless core: transcript, command dispatch, agent feedback, status. |
| `command` | Slash-command framework and the built-in commands. |
| `theme` | Color palettes and lookup helpers. |
| `ui` | Bubble Tea TUI renderer (`ui.Run`). |
| `cmd` | Reference entry point wiring OpenRouter + Playwright MCP. |

Full type and method reference: **[pkg.go.dev/github.com/jjmrocha/ai-chat](https://pkg.go.dev/github.com/jjmrocha/ai-chat)**.

## Configuration

| Variable | Required | Purpose |
|---|---|---|
| `OPEN_ROUTER_KEY` | for OpenRouter | API key passed to `llm.Config.APIKey`. |

ai-toolkit also supports Ollama (no key) and Anthropic — see [ai-toolkit](https://github.com/jjmrocha/ai-toolkit) for provider configuration.

## License

[MIT](LICENSE) © 2026 Joaquim Rocha
