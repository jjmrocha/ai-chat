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
