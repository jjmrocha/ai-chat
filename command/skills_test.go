package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkillsCommand(t *testing.T) {
	t.Run("lists available skills", func(t *testing.T) {
		// given
		ctrl := &mockedSkillsController{
			skillsFunc: func() []string {
				return []string{"brainstorm", "code-review"}
			},
		}
		ctx := &mockedContext{}

		// when
		Skills(ctrl).Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Info, ctx.printed[0].kind)
			assert.Equal(t, "Skills:\n  brainstorm\n  code-review", ctx.printed[0].text)
		}
	})

	t.Run("no skills registered", func(t *testing.T) {
		// given
		ctx := &mockedContext{}

		// when
		Skills(&mockedSkillsController{}).Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Info, ctx.printed[0].kind)
			assert.Equal(t, "No skills registered.", ctx.printed[0].text)
		}
	})
}

func TestSkillCommand(t *testing.T) {
	t.Run("name", func(t *testing.T) {
		assert.Equal(t, "brainstorm", SkillCommand("brainstorm", "Explore an idea").Name())
	})

	t.Run("help is the description", func(t *testing.T) {
		assert.Equal(t, "Explore an idea", SkillCommand("brainstorm", "Explore an idea").Help())
	})

	t.Run("is a prompt", func(t *testing.T) {
		// given
		cmd := SkillCommand("brainstorm", "Explore an idea")

		// when
		prompt, ok := cmd.(Prompt)

		// then
		if assert.True(t, ok) {
			assert.True(t, prompt.Prompt())
		}
	})
}
