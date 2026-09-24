package command

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
