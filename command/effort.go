package command

import (
	"strings"

	"github.com/jjmrocha/ai-toolkit/llm"
)

type effortCmd struct{}

// Effort returns the /effort command: list the reasoning effort levels or
// switch to one.
func Effort() Command {
	return effortCmd{}
}

func (effortCmd) Name() string {
	return "effort"
}

func (effortCmd) Help() string {
	return "/effort [level] Show or switch reasoning effort"
}

// effortLevels are the rungs llm.Effort accepts; the llm package keeps its own
// check unexported, so the command carries the list.
var effortLevels = []string{"off", "low", "medium", "max"}

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
