// Package command defines the pluggable slash-command surface the chat core
// exposes. A Command receives a Context — the narrow set of core capabilities
// it is allowed to use — never the UI. Commands are synchronous; the core runs
// each dispatch on its own goroutine, so a slow command never blocks rendering.
package command

import (
	"context"

	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/mcp"
)

// Kind classifies a transcript line so the UI can style it independently of the
// headless core, which stores only text plus a Kind.
type Kind int

const (
	// User is a line echoing the user's own input.
	User Kind = iota
	// Reply is an assistant reply.
	Reply
	// Info is neutral command or system output.
	Info
	// Error is an error surfaced to the user.
	Error
	// Activity is transient agent activity (tool calls, compaction).
	Activity
)

// AgentController is the slice of agent operations commands may drive. The core
// implements it, supplying the request context so commands stay context-free.
type AgentController interface {
	ChangeModel(name string) error
	ChangeEffort(e llm.Effort)
	AvailableModels() []string
	ModelInfo() *agent.ModelInfo
	Compact()
}

// MCPController is the slice of MCP-manager operations the mcp command drives.
type MCPController interface {
	GetMCPs() []mcp.Status
	Start(ctx context.Context, name string) error
	Stop(name string) error
}

// Context is the capability surface the core hands to a Command.
type Context interface {
	// Agent exposes the agent operations a command may drive.
	Agent() AgentController
	// Print appends a line to the transcript.
	Print(kind Kind, text string)
	// Clear resets the agent session and empties the transcript.
	Clear() error
}

// Command is a slash command registered with the core. Name is matched after
// the leading '/'; Help is a one-liner listed by /help; Run performs the action.
type Command interface {
	Name() string
	Help() string
	Run(ctx Context, args string)
}
