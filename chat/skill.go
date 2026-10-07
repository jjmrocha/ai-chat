package chat

import "github.com/jjmrocha/ai-chat/command"

type skillCommand struct{ name, description string }

func (s skillCommand) Name() string { return s.name }

func (s skillCommand) Help() string { return s.description }

func (skillCommand) Run(command.Context, string) {}
