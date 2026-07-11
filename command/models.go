package command

import "strings"

type modelsCmd struct{}

// Models returns the /models command: list the available models.
func Models() Command          { return modelsCmd{} }
func (modelsCmd) Name() string { return "models" }
func (modelsCmd) Help() string { return "/models         List available models" }
func (modelsCmd) Run(ctx Context, args string) {
	models := ctx.Agent().AvailableModels()
	if len(models) == 0 {
		ctx.Print(Info, "No model list available.")
		return
	}
	ctx.Print(Info, "Models: "+strings.Join(models, ", "))
}
