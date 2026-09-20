# ai-chat

Build terminal chat agents in Go. Bring an [ai-toolkit](https://github.com/jjmrocha/ai-toolkit) agent; get a headless chat core, a Bubble Tea TUI, a pluggable slash-command framework, and MCP server management.

[![Go Reference](https://pkg.go.dev/badge/github.com/jjmrocha/ai-chat.svg)](https://pkg.go.dev/github.com/jjmrocha/ai-chat)
[![Go 1.27+](https://img.shields.io/badge/go-1.27+-00ADD8)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

- **Headless core.** `chat.Chat` runs the transcript and drives the agent with no terminal attached — drive it from a test, a script, or your own UI.
- **Pluggable slash commands.** Ship the built-ins or implement `command.Command` and register your own. No forking.
- **Swappable renderer.** `ui` is one consumer of the core, wired through a single `Observer`. The core stores undecorated text, so a replacement renderer owns every glyph and color.
- **MCP built in.** Register MCP servers and toggle them at runtime with `/mcp`.

## Install

```bash
go get github.com/jjmrocha/ai-chat
```

Requires Go 1.27+.

## Quickstart

A complete terminal chat agent backed by OpenRouter. Drop this into `main.go` in your own module and run it.

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

Real consumers of this library: [joe](https://github.com/jjmrocha/joe), a terminal coding
agent, and [warren](https://github.com/jjmrocha/warren), a financial analyst. Both wire a
model, a toolbox and a skill collection into `chat.New` and hand the result to `ui.Run`,
the way the snippet above does.

## Built-in commands

`/help` and `/exit` are always registered. Everything else is opt-in:
`WithDefaultCommands()` adds the four that need no collaborator, and `/mcp` and `/skills`
have their own options because they take one.

| Option | Command | Effect |
|---|---|---|
| *(always on)* | `/help` | List every registered command |
| *(always on)* | `/exit` | Quit |
| `WithDefaultCommands()` | `/model [name]` | List available models or switch the active one |
| `WithDefaultCommands()` | `/effort [level]` | List reasoning effort levels (`off`, `low`, `medium`, `max`) or switch to one |
| `WithDefaultCommands()` | `/compact` | Force context compaction |
| `WithDefaultCommands()` | `/clear` | Reset conversation |
| `WithMCP(mgr)` | `/mcp [on\|off] [name]` | Show or toggle MCP servers |
| `WithSkills(coll)` | `/skills` | List available skills |

Each of the four defaults is also available individually as `WithModelCommand()`,
`WithEffortCommand()`, `WithCompactCommand()` and `WithClearCommand()`. Registering a
command under an existing name replaces it, so `/help` and `/exit` are overridable like
any other.

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

`ui` is just one `Observer`. The core notifies it via `TranscriptChanged()` / `Quit()`;
everything the UI needs it reads back through `chat.Chat`'s methods.

**The core renders nothing.** A `chat.Line` holds a `Kind` and plain text — no prompt
glyph, no bullet, no indent, no color. Deciding that a user line starts with `❯` is the
renderer's job, which is what makes swapping `ui` for your own front-end worth doing.

**Input is queued, never dropped.** `Submit` returns immediately and the work runs on the
core's own goroutine. Anything sent while a turn is running is queued — the placeholder
reads `(queued)` — and commands and turns share that one queue, so they run strictly in
submission order and never overlap. `Ctrl+C` is the way out mid-turn.

### How the TUI behaves

`ui` renders inline rather than taking over the screen. Each finished transcript line is
printed above a live region holding the thinking row, the title bar, the input and the
status line, so the conversation ends up in the terminal's own scrollback: selection,
copying and wheel scrolling are the terminal's, and work as they do for any other
command's output. The trade-off is that printed lines are never repainted, so resizing
the window leaves earlier markdown wrapped at the old width. `/clear` resets the session
but leaves the conversation in the scrollback, still readable.

Keys: `Enter` sends, `Shift+Enter` (or `Alt+Enter` / `Ctrl+J`) adds a line, `↑` / `↓` walk
prompt history, `Esc` cancels the running turn and drops queued prompts, `Ctrl+C` quits.

### Colors

There is one palette, internal to `ui`, and it cannot be switched — not by the user, not
by an option. A fixed set of colors can only be right for the background it was tuned
against, and neither this library nor the program embedding it can know the user's. So
every value is either an ANSI index, which the terminal's own profile defines, or empty,
meaning the terminal's default text color.

Markdown replies follow the same rule. They use glamour's `dark` layout with every color
replaced by a palette value: bold, italic and strikethrough render as text attributes,
links show underlined text followed by the URL, and fenced code blocks are highlighted in
the 16 basic ANSI colors. Glamour cannot nest styles, so code inside bold or a link loses
the outer style.

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

`command.Context` gives a command the agent (`Agent()`), the transcript (`Print`) and
session reset (`Clear`) — and nothing else. A command needing more than that is handed its
own collaborator at construction, the way `/mcp` and `/skills` are, so no command can
reach a capability it was not given.

`Run` is called on the core's worker goroutine, one command at a time, so it needs no
locking of its own — but a slow `Run` blocks every queued input behind it.

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

Users then toggle servers at runtime: `/mcp on playwright`, `/mcp off playwright`, `/mcp`
to list. With exactly one server registered, the name may be omitted.

> **Security:** `ClientConfig.Command` and `Args` are executed with `os/exec` **without a
> shell**, so they are trusted input. Populate them from operator configuration, never
> from untrusted user input.

### Register skills

A skill is a folder holding a `SKILL.md`. Only the name and description reach the model up
front, as a catalog appended to the system prompt; the body loads on demand when the model
calls `skill_load`.

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

`/skills` lists what is registered. Nothing is discovered automatically — a skill reaches
the model only because `Add` put it there.

> **Security:** a skill folder is trusted input, like an MCP server command. The
> `skill_execute_file` tool runs files the folder ships with the authority and environment
> of the chat process.

### Drive the core headless

No terminal required. `Submit` is asynchronous, though, so a script has to wait for the
turn before reading the transcript — poll `Busy()`:

```go
core := chat.New("CHAT", ag)
core.Submit("hello")

for core.Busy() {
	time.Sleep(10 * time.Millisecond)
}

for _, line := range core.Transcript() {
	fmt.Println(line.Text)
}
```

`Busy()` covers the queue as well as the running turn, so the loop also drains anything
submitted behind it. For anything longer-lived than a script, install an `Observer`
instead of polling.

### Write your own front-end

Implement `chat.Observer`, hand it to the core, and read the transcript back on each
notification. `TranscriptLen()` with `Since(n)` copies only the lines you have not shown
yet, which is what you want when the transcript grows all session.

```go
type printer struct {
	core    *chat.Chat
	printed int
}

func (p *printer) TranscriptChanged() {
	for _, line := range p.core.Since(p.printed) {
		fmt.Println(render(line))
		p.printed++
	}
}

func (p *printer) Quit() { os.Exit(0) }

func render(line chat.Line) string {
	switch line.Kind {
	case command.User:
		return "> " + line.Text
	case command.Activity:
		// Text is the tool call; Detail is its result.
		return "* " + line.Text + "\n    " + line.Detail
	default:
		return line.Text
	}
}

core := chat.New("CHAT", ag)
core.SetContext(context.Background())
core.SetObserver(&printer{core: core})
```

Both `Observer` methods may be called from any goroutine, and from inside a `Chat` method,
so neither should block. `Since` returns nil when `n` is past the end, which is what you
see after `/clear` resets the transcript beneath you — compare against `TranscriptLen()`
and reset your counter.

## Packages

| Package | Purpose |
|---|---|
| `chat` | Headless core: transcript, command dispatch, agent feedback, status. |
| `command` | Slash-command framework and the built-in commands. |
| `ui` | Bubble Tea TUI renderer (`ui.Run`). |

Full type and method reference: **[pkg.go.dev/github.com/jjmrocha/ai-chat](https://pkg.go.dev/github.com/jjmrocha/ai-chat)**.

## Configuration

| Variable | Required | Purpose |
|---|---|---|
| `OPEN_ROUTER_KEY` | for OpenRouter | API key passed to `llm.Config.APIKey`. |

ai-toolkit also supports Ollama (no key) and Anthropic — see
[ai-toolkit](https://github.com/jjmrocha/ai-toolkit) for provider configuration.

## License

[MIT](LICENSE) © 2026 Joaquim Rocha
