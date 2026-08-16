package chat

import (
	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-chat/theme"
)

// Option configures a Chat at construction. Beyond the mandatory name and agent,
// every feature is opt-in through an Option.
type Option func(*Chat)

// WithTheme sets the color palette the UI applies. Defaults to theme.Default.
func WithTheme(t theme.Theme) Option {
	return func(c *Chat) { c.theme = t }
}

// WithCommand registers a custom slash command. The escape hatch for commands
// beyond the built-ins.
func WithCommand(cmd command.Command) Option {
	return func(c *Chat) { c.register(cmd) }
}

// WithModelCommand registers /model.
func WithModelCommand() Option { return WithCommand(command.Model()) }

// WithModelsCommand registers /models.
func WithModelsCommand() Option { return WithCommand(command.Models()) }

// WithEffortCommand registers /effort.
func WithEffortCommand() Option { return WithCommand(command.Effort()) }

// WithCompactCommand registers /compact.
func WithCompactCommand() Option { return WithCommand(command.Compact()) }

// WithClearCommand registers /clear.
func WithClearCommand() Option { return WithCommand(command.Clear()) }

// WithThemeCommand registers /theme.
func WithThemeCommand() Option { return WithCommand(command.Theme()) }

// WithMCP registers /mcp bound to mgr, so the coupling between the command and
// its manager lives in a single option.
func WithMCP(mgr command.MCPController) Option {
	return WithCommand(command.MCP(mgr))
}

// WithDefaultCommands registers every built-in command that needs no external
// dependency: /model, /models, /effort, /compact, /clear and /theme. /mcp
// needs a manager; register it separately with WithMCP.
func WithDefaultCommands() Option {
	return func(c *Chat) {
		for _, cmd := range []command.Command{
			command.Model(),
			command.Models(),
			command.Effort(),
			command.Compact(),
			command.Clear(),
			command.Theme(),
		} {
			c.register(cmd)
		}
	}
}

// WithTelemetryFormatter overrides the per-turn telemetry line formatter.
func WithTelemetryFormatter(f TelemetryFormatter) Option {
	return func(c *Chat) { c.telemetryFmt = f }
}

// WithStatusFormatter overrides the bottom status-bar formatter.
func WithStatusFormatter(f StatusFormatter) Option {
	return func(c *Chat) { c.statusFmt = f }
}
