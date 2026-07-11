package command

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThemeCommand(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		assert.Equal(t, "theme", Theme().Name())
	})

	t.Run("help", func(t *testing.T) {
		assert.NotEmpty(t, Theme().Help())
	})

	t.Run("no args lists themes", func(t *testing.T) {
		// given
		ctx := &mockedContext{}

		// when
		Theme().Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Info, ctx.printed[0].kind)
		}
	})

	t.Run("valid theme switches", func(t *testing.T) {
		// given
		var changed string
		ctx := &mockedContext{
			changeThemeFunc: func(name string) error {
				changed = name
				return nil
			},
		}

		// when
		Theme().Run(ctx, "nord")

		// then
		assert.Equal(t, "nord", changed)
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, "Theme: nord", ctx.printed[0].text)
		}
	})

	t.Run("invalid theme prints error", func(t *testing.T) {
		// given
		ctx := &mockedContext{
			changeThemeFunc: func(name string) error {
				return errors.New("unknown theme \"bogus\"")
			},
		}

		// when
		Theme().Run(ctx, "bogus")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Error, ctx.printed[0].kind)
		}
	})
}
