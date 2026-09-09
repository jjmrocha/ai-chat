package command

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModelCommand(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		assert.Equal(t, "model", Model().Name())
	})

	t.Run("help", func(t *testing.T) {
		assert.NotEmpty(t, Model().Help())
	})

	t.Run("empty args lists available models", func(t *testing.T) {
		// given
		ctx := &mockedContext{
			agentFunc: func() AgentController {
				return &mockedAgentController{
					availableModelsFunc: func() []string {
						return []string{"gpt-4", "claude-3"}
					},
				}
			},
		}

		// when
		Model().Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Info, ctx.printed[0].kind)
			assert.Equal(t, "Models:\n  gpt-4\n  claude-3", ctx.printed[0].text)
		}
	})

	t.Run("empty args with no models available", func(t *testing.T) {
		// given
		ctx := &mockedContext{
			agentFunc: func() AgentController {
				return &mockedAgentController{}
			},
		}

		// when
		Model().Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Info, ctx.printed[0].kind)
			assert.Equal(t, "No models available.", ctx.printed[0].text)
		}
	})

	t.Run("valid model switches and prints", func(t *testing.T) {
		// given
		var changed string
		ctx := &mockedContext{
			agentFunc: func() AgentController {
				return &mockedAgentController{
					changeModelFunc: func(name string) error {
						changed = name
						return nil
					},
				}
			},
		}

		// when
		Model().Run(ctx, "gpt-4")

		// then
		assert.Equal(t, "gpt-4", changed)
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, "Switched to: gpt-4", ctx.printed[0].text)
		}
	})

	t.Run("model error prints error", func(t *testing.T) {
		// given
		ctx := &mockedContext{
			agentFunc: func() AgentController {
				return &mockedAgentController{
					changeModelFunc: func(name string) error {
						return errors.New("unknown model")
					},
				}
			},
		}

		// when
		Model().Run(ctx, "invalid")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Error, ctx.printed[0].kind)
			assert.Equal(t, "Error: unknown model", ctx.printed[0].text)
		}
	})
}
