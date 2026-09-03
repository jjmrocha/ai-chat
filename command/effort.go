package command

import "github.com/jjmrocha/ai-toolkit/llm"

type effortCmd struct{}

// Effort returns the /effort command: set the reasoning effort level.
func Effort() Command          { return effortCmd{} }
func (effortCmd) Name() string { return "effort" }
func (effortCmd) Help() string { return "/effort <level> Set reasoning effort (off, low, medium, max)" }
func (effortCmd) Run(ctx Context, args string) {
	if args == "" {
		ctx.Print(Info, "Usage: /effort off|low|medium|max")
		return
	}
	if err := ctx.Agent().ChangeEffort(llm.Effort(args)); err != nil {
		ctx.Print(Error, "Effort must be: off, low, medium, max")
		return
	}
	ctx.Print(Info, "Effort: "+args)
}
