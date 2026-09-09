# ai-chat

Build terminal chat agents in Go. Bring an [ai-toolkit](https://github.com/jjmrocha/ai-toolkit) agent; get a headless, testable chat core, a Bubble Tea TUI, a pluggable slash-command framework, themes, and MCP server management.

[![Go Reference](https://pkg.go.dev/badge/github.com/jjmrocha/ai-chat.svg)](https://pkg.go.dev/github.com/jjmrocha/ai-chat)
[![Go 1.26+](https://img.shields.io/badge/go-1.26+-00ADD8)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

## Why ai-chat

- **Headless core, not a monolith.** `chat.Chat` runs the transcript and drives the agent with no terminal attached — drive it from a test, a different UI, or the bundled TUI.
- **Slash commands are pluggable.** Ship the built-ins (`/model`, `/effort`, `/mcp`, `/skills`, `/theme`, …) or implement `command.Command` and register your own. No forking required.
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

	ag, err := agent.New(agent.Config{}, client)
	if err != nil {
		log.Fatal(err)
	}
	defer ag.Close()
	ag.StartSession(agent.SessionConfig{Prompt: "You are a helpful assistant."})

	core := chat.New("CHAT", ag, chat.WithDefaultCommands())
	if err := ui.Run(context.Background(), core); err != nil {
		log.Fatal(err)
	}
}
```

Fuller examples live in [`cmd/`](cmd): [`ai-chat`](cmd/ai-chat/main.go) wires a local Ollama model with in-process shell and file packs plus an MCP server, while [`warren`](cmd/warren/main.go) and [`joe`](cmd/joe/main.go) build a financial analyst and a coding agent on OpenRouter.

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
func (pingCmd) Help() string { return "Reply with pong" }
func (pingCmd) Run(ctx command.Context, args string) {
	ctx.Print(command.Info, "pong "+args)
}

core := chat.New("CHAT", ag, chat.WithCommand(pingCmd{}))
```

`Help()` returns the description only — no leading `/ping`, no padding. `/help` builds the
left column from `Name()` and aligns every description to the widest entry, so a command
cannot knock the column out of true.

A command that takes arguments also implements `command.Argumented`, and the spec is
appended after the name:

```go
func (pingCmd) Args() string { return "[message]" }   // renders as: /ping [message]  Reply with pong
```

`Argumented` is optional and detected by type assertion, so the method name and receiver
have to match exactly — a `pingCmd` registered by value whose `Args()` is declared on
`*pingCmd` compiles fine and silently renders as a bare `/ping`.

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

ag, _ := agent.New(agent.Config{}, client)
ag.StartSession(agent.SessionConfig{
	Prompt:  "You are a helpful assistant.",
	ToolBox: toolBox,
})

core := chat.New("CHAT", ag, chat.WithMCP(mcpMng))
```

Users then toggle servers at runtime: `/mcp on playwright`, `/mcp off playwright`, `/mcp` to list.

> **Security:** `ClientConfig.Command` and `Args` are executed with `os/exec` **without a shell**, so they are trusted input. Populate them from operator configuration, never from untrusted user input.

### Register skills

A skill is a folder holding a `SKILL.md`. Only the name and description reach the
model up front, as a catalog appended to the system prompt; the body loads on
demand when the model calls `skill_load`.

```go
skillColl := skills.NewCollection()
if err := skillColl.Add("./skills/stock-research"); err != nil {
	log.Fatal(err)
}

ag, _ := agent.New(agent.Config{}, client)
ag.StartSession(agent.SessionConfig{
	Prompt:  "You are a helpful assistant.",
	ToolBox: toolBox,
	Skills:  skillColl,
})

core := chat.New("CHAT", ag, chat.WithSkills(skillColl))
```

`/skills` lists what is registered. Nothing is discovered automatically — a skill
reaches the model only because `Add` put it there.

> **Security:** a skill folder is trusted input, like an MCP server command. The
> `skill_execute_file` tool runs files the folder ships with the authority and
> environment of the chat process.

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

`WithDefaultCommands()` registers every built-in that needs no external dependency (`/model`, `/effort`, `/compact`, `/clear`). Each is also available as an individual option:

| Constructor | Command | Effect |
|---|---|---|
| `WithModelCommand()` | `/model [name]` | List available models or switch the active one |
| `WithEffortCommand()` | `/effort [level]` | List reasoning effort levels (`off`, `low`, `medium`, `max`) or switch to one |
| `WithClearCommand()` | `/clear` | Reset conversation |
| `WithCompactCommand()` | `/compact` | Force context compaction |
| `WithThemeCommand()` | `/theme [name]` | Show or switch theme |
| `WithMCP(mgr)` | `/mcp [on\|off] [name]` | Show or toggle MCP servers |
| `WithSkills(coll)` | `/skills` | List available skills |

## Packages

| Package | Purpose |
|---|---|
| `chat` | Headless core: transcript, command dispatch, agent feedback, status. |
| `command` | Slash-command framework and the built-in commands. |
| `theme` | Color palettes and lookup helpers. |
| `ui` | Bubble Tea TUI renderer (`ui.Run`). |
| `cmd` | Three reference entry points: `ai-chat` (generic), `warren` (financial analyst), `joe` (coding agent). |

Full type and method reference: **[pkg.go.dev/github.com/jjmrocha/ai-chat](https://pkg.go.dev/github.com/jjmrocha/ai-chat)**.

## Configuration

| Variable | Required | Purpose |
|---|---|---|
| `OPEN_ROUTER_KEY` | for OpenRouter | API key passed to `llm.Config.APIKey`. |

ai-toolkit also supports Ollama (no key) and Anthropic — see [ai-toolkit](https://github.com/jjmrocha/ai-toolkit) for provider configuration.

## License

[MIT](LICENSE) © 2026 Joaquim Rocha
