package command

import (
	"strings"

	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/go-algo/fn"
)

// EffortSwitcher is the model client /effort drives. *llm.LLM satisfies it.
type EffortSwitcher interface {
	// ChangeEffort switches the reasoning effort, returning an error for an
	// unsupported level.
	ChangeEffort(e llm.Effort) error
}

type effortCmd struct{ effort EffortSwitcher }

// Effort returns the /effort command: with no argument it lists the reasoning
// effort levels, otherwise it switches effort to the named one.
func Effort(effort EffortSwitcher) Command {
	return effortCmd{effort: effort}
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

func (c effortCmd) Run(ctx Context, args string) {
	if args == "" {
		ctx.Info(listText("Effort levels", effortLevels))
		return
	}
	if err := c.effort.ChangeEffort(llm.Effort(args)); err != nil {
		ctx.Error("Effort must be: " + strings.Join(effortLevels, ", "))
		return
	}
	ctx.Info("Effort: " + args)
}
