# ai-chat

AI Chat for [ai-toolkit](https://github.com/jjmrocha/ai-toolkit). Terminal-based chat UI with slash commands, theme support, and MCP server management.

## Packages

### `theme` — color palettes

```go
import "github.com/jjmrocha/ai-chat/theme"
```

```go
type Theme struct {
    HeaderName string  // header bar — chat name
    User       string  // user input echo
    Footer     string  // status bar text
    Error      string  // error messages
    Info       string  // info / system output
    Activity   string  // tool calls, compaction events
    Rule       string  // horizontal rule
    TurnSep    string  // turn separator
    Telemetry  string  // per-turn usage / timing line
}
```

Variables: `Default`, `Nord`, `Monokai`, `Catppuccin`.

```go
func ByName(name string) (Theme, bool)  // case-insensitive lookup
func Names() []string                   // returns ["catppuccin", "default", "monokai", "nord"]
```

### `command` — slash command framework

```go
import "github.com/jjmrocha/ai-chat/command"
```

#### Transcript line classification

```go
type Kind int
const (
    User Kind = iota      // echoed user input
    Reply                  // assistant reply
    Info                   // command / system output
    Error                  // error message
    Activity               // tool calls, compaction
    Telemetry              // per-turn usage line
)
```

#### Interfaces

```go
type Command interface {
    Name() string
    Help() string
    Run(ctx Context, args string)
}

type Context interface {
    Agent() AgentController
    Print(kind Kind, text string)
    Clear() error
    ChangeTheme(name string) error
}

type AgentController interface {
    ChangeModel(name string) error
    ChangeEffort(e llm.Effort)
    AvailableModels() []string
    ModelInfo() *agent.ModelInfo
    Compact()
}

type MCPController interface {
    GetMCPs() []mcp.Status
    Start(ctx context.Context, name string) error
    Stop(name string) error
}
```

#### Available commands

| Constructor | Command | Effect |
|---|---|---|
| `Model()` | `/model <name>` | Switch active model |
| `Models()` | `/models` | List available models |
| `Effort()` | `/effort <level>` | Set reasoning effort (off/low/medium/max) |
| `Clear()` | `/clear` | Reset conversation |
| `Compact()` | `/compact` | Force context compaction |
| `Theme()` | `/theme [name]` | Show or switch theme |
| `MCP(mgr)` | `/mcp [on\|off] [name]` | Show or toggle MCP servers |

### `chat` — headless core

```go
import "github.com/jjmrocha/ai-chat/chat"
```

#### Types

```go
type Line struct {
    Kind command.Kind
    Text string
}

type StatusInfo struct {
    Name     string        // model name
    Provider llm.Provider  // provider
    Effort   llm.Effort    // reasoning effort level
    CtxPct   float64       // context window usage %
    Tokens   int           // total tokens used
}
```

#### Interfaces

```go
type Observer interface {
    TranscriptChanged()  // new transcript lines — re-render
    Quit()               // core requests program exit
}
```

#### Function types

```go
type Option func(*Chat)
type TelemetryFormatter func(agent.Metadata) string
type StatusFormatter func(StatusInfo) string
```

#### Constructor

```go
func New(name string, ag *agent.Agent, opts ...Option) *Chat
```

Options: `WithTheme`, `WithCommand`, `WithModelCommand`, `WithModelsCommand`, `WithEffortCommand`, `WithCompactCommand`, `WithClearCommand`, `WithThemeCommand`, `WithMCP`, `WithTelemetryFormatter`, `WithStatusFormatter`.

#### Chat methods

| Method | Signature | Description |
|---|---|---|
| `Name` | `() string` | Display name |
| `Theme` | `() theme.Theme` | Active palette |
| `SetObserver` | `(Observer)` | Register observer |
| `Transcript` | `() []Line` | Snapshot of transcript |
| `Busy` | `() bool` | Agent turn in flight? |
| `LastMetadata` | `() agent.Metadata` | Last turn's metadata |
| `Submit` | `(string)` | Process user input |
| `Status` | `() StatusInfo` | Current status |
| `StatusText` | `() string` | Rendered status bar |
| `Print` | `(command.Kind, string)` | Append line |
| `Clear` | `() error` | Reset session + transcript |
| `ChangeTheme` | `(string) error` | Switch palette |
| `ChangeModel` | `(string) error` | Switch model |
| `ChangeEffort` | `(llm.Effort)` | Set reasoning effort |
| `AvailableModels` | `() []string` | List models |
| `ModelInfo` | `() *agent.ModelInfo` | Current model |
| `Compact` | `()` | Force compaction |

Agent feedback methods: `ToolCalled`, `ContextCompacted`, `ContextCompactionFailed`, `SessionReset`, `SessionStarted`, `SessionClosed`.

### `ui` — terminal renderer

```go
import "github.com/jjmrocha/ai-chat/ui"
```

```go
func Run(ctx context.Context, core *chat.Chat) error
```

Launches a Bubble Tea TUI program. Blocks until the user quits or `ctx` is cancelled.

### `cmd` — entry point (package `main`)

Requires `OPEN_ROUTER_KEY`. Registers all built-in commands and the Playwright MCP server.

```bash
export OPEN_ROUTER_KEY=sk-...
go run ./cmd
```
