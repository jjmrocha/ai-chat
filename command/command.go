// Package command defines the pluggable slash-command surface the chat core
// exposes. A Command receives a Context — the narrow set of core capabilities
// it is allowed to use — never the UI.
package command

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

// Context is the capability surface the chat core hands to a Command.
type Context interface {
	// Print appends a line to the transcript.
	Print(kind Kind, text string)
}
