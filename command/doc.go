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
