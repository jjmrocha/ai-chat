package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompactCommand(t *testing.T) {
	t.Run("calls Compact on agent", func(t *testing.T) {
		// given
		var compacted bool
		ctx := &mockedContext{
			agentFunc: func() AgentController {
				return &mockedAgentController{
					compactFunc: func() {
						compacted = true
					},
				}
			},
		}

		// when
		Compact().Run(ctx, "")

		// then
		assert.True(t, compacted)
		assert.Empty(t, ctx.printed)
	})
}
