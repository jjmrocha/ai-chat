package command

type modelCmd struct{}

// Model returns the /model command: with no argument it lists the available
// models, otherwise it switches to the named one.
func Model() Command {
	return modelCmd{}
}

func (modelCmd) Name() string {
	return "model"
}

func (modelCmd) Help() string {
	return "Show or switch model"
}

func (modelCmd) Args() string {
	return "[name]"
}

func (modelCmd) Run(ctx Context, args string) {
	if args == "" {
		printList(ctx, "Models", "No models available.", ctx.Agent().AvailableModels())
		return
	}
	if err := ctx.Agent().ChangeModel(args); err != nil {
		printErr(ctx, err)
		return
	}
	ctx.Print(Info, "Switched to: "+args)
}
