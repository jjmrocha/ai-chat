package chat

import (
	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/llm"
)

// Line is one entry in the transcript: the text to show and the [command.Kind]
// a front-end styles it by. Text carries no decoration — no prompt glyph, no
// bullet, no indent — so a renderer is free to present it however it likes.
//
// Detail is the second half of a paired entry and is empty for most kinds. For
// [command.Activity] it holds the tool result belonging to the call in Text, so
// a renderer can style request and response differently without parsing.
type Line struct {
	Kind   command.Kind
	Text   string
	Detail string
}

// StatusInfo is the state a status line is built from: the active model and
// provider, the reasoning effort, how full the context window is as a
// percentage, and the token total of the last turn. Fields are zero when the
// model's limits are not known yet.
type StatusInfo struct {
	Name     string
	Provider llm.Provider
	Effort   llm.Effort
	CtxPct   float64
	Tokens   int
}
