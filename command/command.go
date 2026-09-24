package command

import (
	"context"

	"github.com/jjmrocha/ai-toolkit/llm"
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

	// Context is cancelled when the user cancels the running command or the
	// session closes. Pass it to anything the command waits on, so a slow
	// command can be interrupted.
	Context() context.Context
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
