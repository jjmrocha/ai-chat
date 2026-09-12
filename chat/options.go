package chat

import (
	"github.com/jjmrocha/ai-chat/command"
)

// Option configures a [Chat] at construction. Options are applied in order, so
// a later one may replace what an earlier one registered.
type Option func(*Chat)

// WithCommand registers a slash command, replacing any command already
// registered under the same name — including the built-in /help and /exit.
func WithCommand(cmd command.Command) Option {
	return func(c *Chat) { c.register(cmd) }
}

// WithModelCommand registers /model, which lists the available models or
// switches the active one.
func WithModelCommand() Option { return WithCommand(command.Model()) }

// WithEffortCommand registers /effort, which lists the reasoning effort levels
// or switches to one.
func WithEffortCommand() Option { return WithCommand(command.Effort()) }

// WithCompactCommand registers /compact, which forces context compaction.
func WithCompactCommand() Option { return WithCommand(command.Compact()) }

// WithClearCommand registers /clear, which resets the conversation.
func WithClearCommand() Option { return WithCommand(command.Clear()) }

// WithMCP registers /mcp, which lists the MCP servers mgr knows about and
// toggles them at runtime.
func WithMCP(mgr command.MCPController) Option {
	return WithCommand(command.MCP(mgr))
}

// WithSkills registers /skills, which lists the skills in coll.
func WithSkills(coll command.SkillsController) Option {
	return WithCommand(command.Skills(coll))
}

// WithDefaultCommands registers every built-in command that needs no external
// dependency: /model, /effort, /compact and /clear. /mcp and /skills take a
// collaborator and so have their own options.
func WithDefaultCommands() Option {
	return func(c *Chat) {
		for _, opt := range []Option{
			WithModelCommand(),
			WithEffortCommand(),
			WithCompactCommand(),
			WithClearCommand(),
		} {
			opt(c)
		}
	}
}

// WithTelemetryFormatter replaces the formatter that renders the per-turn
// telemetry line. Returning an empty string suppresses the line.
func WithTelemetryFormatter(f TelemetryFormatter) Option {
	return func(c *Chat) { c.telemetryFmt = f }
}

// WithStatusFormatter replaces the formatter that renders the status line a
// front-end shows below the input.
func WithStatusFormatter(f StatusFormatter) Option {
	return func(c *Chat) { c.statusFmt = f }
}
