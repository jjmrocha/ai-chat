package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompactCommand(t *testing.T) {
	t.Run("compacts the session and prints nothing itself", func(t *testing.T) {
		// given
		var compacted bool
		ctx := &mockedContext{
			compactFunc: func() { compacted = true },
		}

		// when
		Compact().Run(ctx, "")

		// then
		assert.True(t, compacted)
		assert.Empty(t, ctx.printed)
	})
}
