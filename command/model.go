package command

import (
	"context"

	"github.com/jjmrocha/ai-toolkit/llm"
)

// ModelSwitcher is the model client /model drives. *llm.LLM satisfies it.
type ModelSwitcher interface {
	// AvailableModels lists the models that can be switched to. The active
	// model is always included.
	AvailableModels() []string

	// ChangeModel switches to the named model, which must be one of
	// AvailableModels, and returns its details. It returns the client's error
	// on failure and keeps the current model.
	ChangeModel(ctx context.Context, model string) (*llm.ModelInfo, error)
}

type modelCmd struct{ models ModelSwitcher }

// Model returns the /model command: with no argument it lists the models in
// models, otherwise it switches to the named one.
func Model(models ModelSwitcher) Command {
	return modelCmd{models: models}
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

func (m modelCmd) Run(ctx Context, args string) {
	if args == "" {
		printList(ctx, "Models", "No models available.", m.models.AvailableModels())
		return
	}
	if _, err := m.models.ChangeModel(ctx.Context(), args); err != nil {
		printErr(ctx, err)
		return
	}
	ctx.Info("Switched to: " + args)
}
