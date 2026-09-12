package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExitCommand(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		assert.Equal(t, "exit", Exit(&mockedQuitter{}).Name())
	})

	t.Run("help", func(t *testing.T) {
		assert.NotEmpty(t, Exit(&mockedQuitter{}).Help())
	})

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

	t.Run("arguments are ignored", func(t *testing.T) {
		// given
		q := &mockedQuitter{}

		// when
		Exit(q).Run(&mockedContext{}, "now")

		// then
		assert.Equal(t, 1, q.quits)
	})
}
