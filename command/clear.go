package command

type clearCmd struct{}

// Clear returns the /clear command: reset the conversation.
func Clear() Command          { return clearCmd{} }
func (clearCmd) Name() string { return "clear" }
func (clearCmd) Help() string { return "/clear          Reset conversation" }
func (clearCmd) Run(ctx Context, _ string) {
	if err := ctx.Clear(); err != nil {
		ctx.Print(Error, "Error: "+err.Error())
		return
	}
	ctx.Print(Info, "Context cleared.")
}
