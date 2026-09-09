package command

import (
	"testing"

	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/stretchr/testify/assert"
)

func TestEffortCommand(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		assert.Equal(t, "effort", Effort().Name())
	})

	t.Run("help", func(t *testing.T) {
		assert.NotEmpty(t, Effort().Help())
	})

	t.Run("empty args lists effort levels", func(t *testing.T) {
		// given
		ctx := &mockedContext{}

		// when
		Effort().Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Info, ctx.printed[0].kind)
			assert.Equal(t, "Effort levels:\n  off\n  low\n  medium\n  max", ctx.printed[0].text)
		}
	})

	t.Run("valid effort levels", func(t *testing.T) {
		levels := []string{"off", "low", "medium", "max"}
		for _, level := range levels {
			t.Run(level, func(t *testing.T) {
				// given
				var changed llm.Effort
				ctx := &mockedContext{
					agentFunc: func() AgentController {
						return &mockedAgentController{
							changeEffortFunc: func(e llm.Effort) error {
								changed = e
								return nil
							},
						}
					},
				}

				// when
				Effort().Run(ctx, level)

				// then
				assert.Equal(t, llm.Effort(level), changed)
				if assert.Len(t, ctx.printed, 1) {
					assert.Equal(t, "Effort: "+level, ctx.printed[0].text)
				}
			})
		}
	})

	t.Run("rejected effort prints error", func(t *testing.T) {
		// given
		var changed llm.Effort
		ctx := &mockedContext{
			agentFunc: func() AgentController {
				return &mockedAgentController{
					changeEffortFunc: func(e llm.Effort) error {
						changed = e
						return llm.ErrInvalidEffort
					},
				}
			},
		}

		// when
		Effort().Run(ctx, "extreme")

		// then
		assert.Equal(t, llm.Effort("extreme"), changed, "the level is delegated, not validated locally")
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Error, ctx.printed[0].kind)
			assert.Equal(t, "Effort must be: off, low, medium, max", ctx.printed[0].text)
		}
	})

	t.Run("empty args does not reach the agent", func(t *testing.T) {
		// given
		ctx := &mockedContext{
			agentFunc: func() AgentController {
				return &mockedAgentController{
					changeEffortFunc: func(llm.Effort) error {
						t.Error("ChangeEffort should not be called")
						return nil
					},
				}
			},
		}

		// when
		Effort().Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Info, ctx.printed[0].kind)
		}
	})
}
