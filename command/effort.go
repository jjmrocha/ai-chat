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
	switch llm.Effort(args) {
	case llm.EffortOff, llm.EffortLow, llm.EffortMedium, llm.EffortMax:
		ctx.Agent().ChangeEffort(llm.Effort(args))
		ctx.Print(Info, "Effort: "+args)
	default:
		ctx.Print(Error, "Effort must be: off, low, medium, max")
	}
}
