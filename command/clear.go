package command

type clearCmd struct{}

// Clear returns the /clear command, which resets the conversation. The
// transcript already shown stays in the terminal's scrollback.
func Clear() Command {
	return clearCmd{}
}

func (clearCmd) Name() string {
	return "clear"
}

func (clearCmd) Help() string {
	return "Reset conversation"
}

func (clearCmd) Run(ctx Context, _ string) {
	if err := ctx.Clear(); err != nil {
		printErr(ctx, err)
		return
	}

	ctx.Print(Info, "Context cleared.")
}
