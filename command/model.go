package command

type modelCmd struct{}

// Model returns the /model command: switch the active model.
func Model() Command          { return modelCmd{} }
func (modelCmd) Name() string { return "model" }
func (modelCmd) Help() string { return "/model <name>   Switch model" }
func (modelCmd) Run(ctx Context, args string) {
	if args == "" {
		ctx.Print(Info, "Usage: /model <name>")
		return
	}
	if err := ctx.Agent().ChangeModel(args); err != nil {
		ctx.Print(Error, "Error: "+err.Error())
		return
	}
	ctx.Print(Info, "Switched to: "+args)
}
