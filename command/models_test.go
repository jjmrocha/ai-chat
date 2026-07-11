package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModelsCommand(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		assert.Equal(t, "models", Models().Name())
	})

	t.Run("help", func(t *testing.T) {
		assert.NotEmpty(t, Models().Help())
	})

	t.Run("lists available models", func(t *testing.T) {
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
		Models().Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Info, ctx.printed[0].kind)
			assert.Equal(t, "Models: gpt-4, claude-3", ctx.printed[0].text)
		}
	})

	t.Run("no models available", func(t *testing.T) {
		// given
		ctx := &mockedContext{
			agentFunc: func() AgentController {
				return &mockedAgentController{}
			},
		}

		// when
		Models().Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, "No model list available.", ctx.printed[0].text)
		}
	})
}
