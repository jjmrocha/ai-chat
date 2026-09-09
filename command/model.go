package command

type modelCmd struct{}

// Model returns the /model command: list the available models or switch to one.
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
		models := ctx.Agent().AvailableModels()
		if len(models) == 0 {
			ctx.Print(Info, "No models available.")
			return
		}
		ctx.Print(Info, listText("Models", models))
		return
	}
	if err := ctx.Agent().ChangeModel(args); err != nil {
		ctx.Print(Error, "Error: "+err.Error())
		return
	}
	ctx.Print(Info, "Switched to: "+args)
}
