# ai-chat

[![Go Reference](https://pkg.go.dev/badge/github.com/jjmrocha/ai-chat.svg)](https://pkg.go.dev/github.com/jjmrocha/ai-chat)
![Go](https://img.shields.io/badge/go-1.27.1%2B-00ADD8?logo=go)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

**Terminal chat agents in Go: bring an [ai-toolkit](https://github.com/jjmrocha/ai-toolkit)
agent, and get a terminal UI with slash commands.**

I built this library for my own agents, and it follows my own preferences.
More mature libraries exist. If this one fits your needs, use it.

```bash
go get github.com/jjmrocha/ai-chat
```

## Highlights

- 💬 **An inline terminal UI**: the conversation goes into the terminal's own scrollback.
- ⌨️ **Slash commands**: built-in commands for models, effort, compaction, MCP servers and skills, and your own commands.
- 📥 **No lost input**: messages that you send while the agent works wait in a queue.
- ⏹️ **Cancel at any time**: `Esc` stops the running turn and clears the queue.
- 👥 **Sub-agent progress**: see the tool calls and token count of a running sub-agent.
- 🧱 **A headless core**: run the chat from a test or a script, without a terminal.
- 🛡️ **Safe output**: model and tool output cannot send escape sequences to your terminal.

## Quick start

A terminal chat agent on a local Ollama model:

```go
func run(ctx context.Context) error {
	modelCfg := llm.Config{
		Provider: llm.ProviderOllama,
		Model:    "qwen3:8b",
	}

	model, err := llm.New(modelCfg)
	if err != nil {
		return err
	}

	agentCfg := agent.Config{MaxIterations: 10}

	agt, err := agent.New(agentCfg, model)
	if err != nil {
		return err
	}
	defer agt.Close()

	core := chat.New("CHAT", agt, chat.WithDefaultCommands())

	session := agent.SessionConfig{Prompt: "You are a helpful assistant."}
	agt.StartSession(session)

	return ui.Run(ctx, core)
}
```

- Call `chat.New` before `StartSession`. The agent reports a resumed session only to the chat that exists when the session starts.
- [joe](https://github.com/jjmrocha/joe), a coding agent, and [warren](https://github.com/jjmrocha/warren), a financial analyst, are built this way.

## Packages

| Package | What it does |
| --- | --- |
| [`chat`](#chat) | The headless core: transcript, input queue, commands and progress |
| [`command`](#command) | The slash-command interface and the built-in commands |
| [`ui`](#ui) | The terminal UI that shows a `chat.Chat` |

The full API reference is on
[pkg.go.dev](https://pkg.go.dev/github.com/jjmrocha/ai-chat).

### `chat`

```go
core := chat.New("CHAT", agt,
	chat.WithDefaultCommands(),
	chat.WithMCP(mcpManager),
	chat.WithSkills(collection),
	chat.WithSkillCommand("git-release", "Draft release notes"),
	chat.WithSubagents(subagentManager),
)
```

- `New` installs the chat as the agent's `agent.Feedback`. Use each agent for one chat only.
- `Submit` returns at once. Turns and commands run one at a time, in the order that they arrive.
- `Cancel` stops the running turn or command and removes the queued input.
- `WithSubagents` shows the work of the running sub-agent in `Progress`. It replaces the feedback that is set on the manager.
- `WithTelemetryFormatter` changes the line after each turn. `WithStatusFormatter` changes the status line.
- The core removes escape sequences and control characters from all text, but keeps newlines and tabs.

To use the chat without a terminal, poll `Busy` until the turn ends:

```go
core := chat.New("CHAT", agt)
agt.StartSession(session)

core.Submit("hello")

for core.Busy() {
	time.Sleep(10 * time.Millisecond)
}

for _, line := range core.Transcript() {
	fmt.Println(line.Text)
}
```

- `Busy` is true while a turn runs and while input waits in the queue.
- For a long-running program, install an `Observer` with `SetObserver` instead of polling.
- Call `Close` when you are done. It cancels the running work and waits for it to stop.

### `command`

| Option | Command | What it does |
| --- | --- | --- |
| *(always)* | `/help` | Lists every registered command |
| *(always)* | `/exit` | Quits |
| `WithDefaultCommands` | `/model [name]` | Lists the models, or changes to the named model |
| `WithDefaultCommands` | `/effort [level]` | Lists the effort levels (`off`, `low`, `medium`, `max`), or changes to one |
| `WithDefaultCommands` | `/compact` | Summarizes the older turns now |
| `WithDefaultCommands` | `/clear` | Starts a new conversation |
| `WithMCP(mgr)` | `/mcp [on\|off] [name]` | Lists the MCP servers, or starts or stops one |
| `WithSkills(coll)` | `/skills` | Lists the skills |
| `WithSkillCommand(name, desc)` | `/name [text]` | Sends the input, as typed, to the agent |

To add your own command, implement `command.Command`:

```go
type pingCmd struct{}

func (pingCmd) Name() string { return "ping" }

func (pingCmd) Help() string { return "Reply with pong" }

func (pingCmd) Run(ctx command.Context, args string) {
	ctx.Info("pong " + args)
}
```

```go
core := chat.New("CHAT", agt, chat.WithCommand(pingCmd{}))
```

- `Help` returns only the description. `/help` adds the name and aligns the columns.
- To show arguments in `/help`, also implement `command.Argumented`. Declare `Args` on the same receiver type that you register.
- `command.Context` gives a command the agent, the transcript, a session reset and a cancellation context.
- If a command needs more, give it to the command when you create it. `/mcp` and `/skills` do this.
- `Run` blocks the queue until it returns. Pass `ctx.Context()` to everything that it waits on, so `Esc` can stop it.
- A command with the same name replaces the earlier one. This includes `/help` and `/exit`.
- Input that starts with `/` but names no command goes to the agent as a normal message.
- `WithDefaultCommands` adds four commands. To add only one, use `WithModelCommand`, `WithEffortCommand`, `WithCompactCommand` or `WithClearCommand`.
- With one MCP server, `/mcp on` and `/mcp off` need no server name.
- A skill command does not run code. The agent gets the input, for example `/git-release v1.2`, and loads the skill itself.

### `ui`

```go
if err := ui.Run(ctx, core); err != nil {
	return err
}
```

| Key | What it does |
| --- | --- |
| `Enter` | Sends the input |
| `Shift+Enter`, `Alt+Enter` or `Ctrl+J` | Adds a line |
| `↑` / `↓` | Shows the previous or next input |
| `Esc` | Stops the running turn or command, and clears the queue |
| `Ctrl+C` | Quits |

- The UI draws inline, not on a full screen. Finished lines go into the terminal's scrollback, so you select, copy and scroll them as usual.
- The UI does not draw printed lines again. After you resize the window, earlier markdown keeps its old width.
- Type `/` at the start of the input to list up to three matching commands. `↑` and `↓` select, `Enter` runs, `Tab` fills in, and `Esc` closes the list.
- While a sub-agent runs, the row under the thinking row shows its tool calls and latest token count.
- The colors are ANSI color indexes, so your terminal's theme sets them. You cannot change the palette.
- Code blocks in replies use the 16 basic ANSI colors. Code in bold text or in a link loses the outer style.
- `Run` sets the observer and the context of the core. It closes the core before it returns.

## Security

ai-chat runs the MCP servers and skill scripts that you register, with your
program's authority over the whole filesystem. `/mcp` starts an MCP server's
`Command` without a shell. A skill's `skill_execute_file` tool runs the files in
the skill folder. MCP servers and skill scripts get a filtered environment, but
they can still read credential files on disk.

Take MCP commands and skill folders from your own configuration. Never take
them from untrusted input.

The core removes escape sequences and control characters from all model and
tool output. That output therefore cannot change the clipboard, the window
title or the target of a link.

## License

MIT, see [LICENSE](LICENSE).
