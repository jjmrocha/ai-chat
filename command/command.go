// Package command is the slash-command framework for a terminal chat agent,
// together with the built-in commands.
//
// A command is any value implementing [Command]; register one with
// chat.WithCommand and it is dispatched like a built-in, with no forking. A
// command that takes arguments also implements [Argumented] so /help can show
// its usage.
//
// Commands never touch the chat core directly. They receive a [Context], which
// exposes the transcript, session reset and the agent; anything narrower — MCP
// servers, the skill catalog, the registry, the quit signal — is injected into
// the individual command at construction, so no command can reach a capability
// it was not given.
package command

import (
	"context"

	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/mcp"
)

// Kind classifies a transcript line so a front-end can style it. The core
// stores plain text and never decorates it; choosing a color, glyph or layout
// per Kind is the renderer's job.
type Kind int

// The kinds of transcript line a front-end may be asked to render.
const (
	// User is input the person typed.
	User Kind = iota

	// Reply is the agent's answer, conventionally rendered as markdown.
	Reply

	// Info is command output, such as a list of models.
	Info

	// Error is a failed turn or command.
	Error

	// Activity is a tool call. Its Detail holds the result.
	Activity

	// Telemetry is the per-turn timing and token summary.
	Telemetry
)

// AgentController is the slice of the agent that commands may drive: switching
// model or effort, listing models, and forcing compaction. Obtain one from
// [Context.Agent].
type AgentController interface {
	// ChangeModel switches to the named model, which must be one of
	// AvailableModels. It returns the agent's error on failure.
	ChangeModel(name string) error

	// ChangeEffort switches the reasoning effort, returning an error for an
	// unsupported level.
	ChangeEffort(e llm.Effort) error

	// AvailableModels lists the models that can be switched to. The active
	// model is always included.
	AvailableModels() []string

	// Compact forces context compaction. It reports progress through the
	// transcript rather than returning it.
	Compact()
}

// MCPController is the MCP server manager /mcp drives. Pass one to [MCP], or
// to chat.WithMCP, which wires it for you.
type MCPController interface {
	// Status lists every registered server and whether it is running.
	Status() []mcp.Status

	// Start launches the named server.
	Start(ctx context.Context, name string) error

	// Stop shuts the named server down.
	Stop(name string) error
}

// SkillsController is the skill catalog /skills reads. Pass one to [Skills], or
// to chat.WithSkills, which wires it for you.
type SkillsController interface {
	// Skills lists the names of the registered skills.
	Skills() []string
}

// Context is what a running command may do to the session. It is deliberately
// narrow: a command that needs more than this is given its own collaborator at
// construction instead.
type Context interface {
	// Agent returns the controller for model, effort and compaction.
	Agent() AgentController

	// Print appends a line to the transcript. Pass undecorated text; the
	// front-end styles it by kind.
	Print(kind Kind, text string)

	// Clear resets the conversation, returning the agent's error if the
	// session could not be reset.
	Clear() error
}

// Command is a slash command. Register an implementation with
// chat.WithCommand to have it dispatched like a built-in.
//
// Run is called on the core's worker goroutine, one command at a time, so an
// implementation need not guard against concurrent runs — but a slow Run blocks
// every queued input behind it.
type Command interface {
	// Name is the command word without its leading slash, such as "model".
	Name() string

	// Help is the one-line description /help shows. Return the description
	// only: no leading slash, no name, no padding.
	Help() string

	// Run executes the command. args is everything after the command word,
	// trimmed, and is empty when nothing followed it.
	Run(ctx Context, args string)
}

// Argumented is the optional half of [Command] for commands that take
// arguments. /help appends the spec after the name, so returning "[name]" from
// a command called "model" renders as "/model [name]".
//
// It is detected by type assertion, so the receiver must match the value that
// was registered: declaring Args on *T while registering a T silently renders
// as a bare name.
type Argumented interface {
	// Args is the argument spec shown in /help, such as "[on|off] [name]".
	// Return an empty string to render the name alone.
	Args() string
}
