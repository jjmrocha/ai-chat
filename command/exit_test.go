package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExitCommand(t *testing.T) {
	t.Run("run quits exactly once and prints nothing", func(t *testing.T) {
		// given
		q := &mockedQuitter{}
		ctx := &mockedContext{}

		// when
		Exit(q).Run(ctx, "")

		// then
		assert.Equal(t, 1, q.quits)
		assert.Empty(t, ctx.printed)
	})

}
