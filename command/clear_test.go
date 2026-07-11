package command

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClearCommand(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		assert.Equal(t, "clear", Clear().Name())
	})

	t.Run("help", func(t *testing.T) {
		assert.NotEmpty(t, Clear().Help())
	})

	t.Run("clear succeeds", func(t *testing.T) {
		// given
		var cleared bool
		ctx := &mockedContext{
			clearFunc: func() error {
				cleared = true
				return nil
			},
		}

		// when
		Clear().Run(ctx, "")

		// then
		assert.True(t, cleared)
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, "Context cleared.", ctx.printed[0].text)
		}
	})

	t.Run("clear error prints error", func(t *testing.T) {
		// given
		ctx := &mockedContext{
			clearFunc: func() error {
				return errors.New("session error")
			},
		}

		// when
		Clear().Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Error, ctx.printed[0].kind)
			assert.Equal(t, "Error: session error", ctx.printed[0].text)
		}
	})
}
