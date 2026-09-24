package command

import (
	"strings"

	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/go-algo/fn"
)

type effortCmd struct{}

// Effort returns the /effort command: with no argument it lists the reasoning
// effort levels, otherwise it switches to the named one.
func Effort() Command {
	return effortCmd{}
}

func (effortCmd) Name() string {
	return "effort"
}

func (effortCmd) Help() string {
	return "Show or switch reasoning effort"
}

func (effortCmd) Args() string {
	return "[level]"
}

var effortLevels = fn.Map(
	[]llm.Effort{llm.EffortOff, llm.EffortLow, llm.EffortMedium, llm.EffortMax},
	func(e llm.Effort) string { return string(e) },
)

func (effortCmd) Run(ctx Context, args string) {
	if args == "" {
		ctx.Print(Info, listText("Effort levels", effortLevels))
		return
	}
	if err := ctx.Agent().ChangeEffort(llm.Effort(args)); err != nil {
		ctx.Print(Error, "Effort must be: "+strings.Join(effortLevels, ", "))
		return
	}
	ctx.Print(Info, "Effort: "+args)
}
