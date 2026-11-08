package command

import (
	"context"
	"errors"
	"testing"

	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/stretchr/testify/assert"
)

func TestModelCommand(t *testing.T) {
	t.Run("empty args lists available models", func(t *testing.T) {
		// given
		models := &mockedModelSwitcher{
			availableModelsFunc: func() []string {
				return []string{"gpt-4", "claude-3"}
			},
		}
		ctx := &mockedContext{}

		// when
		Model(models).Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, infoStream, ctx.printed[0].kind)
			assert.Equal(t, "Models:\n  gpt-4\n  claude-3", ctx.printed[0].text)
		}
	})

	t.Run("empty args with no models available", func(t *testing.T) {
		// given
		ctx := &mockedContext{}

		// when
		Model(&mockedModelSwitcher{}).Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, infoStream, ctx.printed[0].kind)
			assert.Equal(t, "No models available.", ctx.printed[0].text)
		}
	})

	t.Run("valid model switches and prints", func(t *testing.T) {
		// given
		var changed string
		models := &mockedModelSwitcher{
			changeModelFunc: func(_ context.Context, name string) (*llm.ModelInfo, error) {
				changed = name
				return &llm.ModelInfo{Name: name}, nil
			},
		}
		ctx := &mockedContext{}

		// when
		Model(models).Run(ctx, "gpt-4")

		// then
		assert.Equal(t, "gpt-4", changed)
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, "Switched to: gpt-4", ctx.printed[0].text)
		}
	})

	t.Run("switch uses the command context", func(t *testing.T) {
		// given
		type key struct{}
		cmdCtx := context.WithValue(t.Context(), key{}, "command")
		var used context.Context
		models := &mockedModelSwitcher{
			changeModelFunc: func(ctx context.Context, name string) (*llm.ModelInfo, error) {
				used = ctx
				return &llm.ModelInfo{Name: name}, nil
			},
		}
		ctx := &mockedContext{ctx: cmdCtx}

		// when
		Model(models).Run(ctx, "gpt-4")

		// then
		if assert.NotNil(t, used) {
			assert.Equal(t, "command", used.Value(key{}))
		}
	})

	t.Run("model error prints error", func(t *testing.T) {
		// given
		models := &mockedModelSwitcher{
			changeModelFunc: func(context.Context, string) (*llm.ModelInfo, error) {
				return nil, errors.New("unknown model")
			},
		}
		ctx := &mockedContext{}

		// when
		Model(models).Run(ctx, "invalid")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, errorStream, ctx.printed[0].kind)
			assert.Equal(t, "Error: unknown model", ctx.printed[0].text)
		}
	})
}
