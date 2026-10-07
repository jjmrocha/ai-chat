package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkillsCommand(t *testing.T) {
	t.Run("lists available skills", func(t *testing.T) {
		// given
		ctrl := &mockedSkillCatalog{
			skillsFunc: func() []string {
				return []string{"brainstorm", "code-review"}
			},
		}
		ctx := &mockedContext{}

		// when
		Skills(ctrl).Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, infoStream, ctx.printed[0].kind)
			assert.Equal(t, "Skills:\n  brainstorm\n  code-review", ctx.printed[0].text)
		}
	})

	t.Run("no skills registered", func(t *testing.T) {
		// given
		ctx := &mockedContext{}

		// when
		Skills(&mockedSkillCatalog{}).Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, infoStream, ctx.printed[0].kind)
			assert.Equal(t, "No skills registered.", ctx.printed[0].text)
		}
	})
}
