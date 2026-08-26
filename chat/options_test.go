package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithDefaultCommands(t *testing.T) {
	// given
	c := newChat("test", WithDefaultCommands())

	// when
	result := c.helpText()

	// then
	for _, name := range []string{"/model", "/models", "/effort", "/compact", "/clear", "/theme"} {
		assert.Contains(t, result, name)
	}
}

type stubSkills struct{ names []string }

func (s stubSkills) Skills() []string { return s.names }

func TestWithSkills(t *testing.T) {
	// given
	c := newChat("test", WithSkills(stubSkills{names: []string{"brainstorm"}}))

	// when
	result := c.helpText()

	// then
	assert.Contains(t, result, "/skills")
}
