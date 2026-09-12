package chat

import (
	"testing"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/stretchr/testify/assert"
)

func commandNames(c *Chat) []string {
	cmds := c.Commands()
	names := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		names = append(names, cmd.Name())
	}
	return names
}

func TestHelpAndExitAreRegisteredByDefault(t *testing.T) {
	// given

	// when
	result := commandNames(newChat("test"))

	// then
	assert.Equal(t, []string{"exit", "help"}, result)
}

func TestWithDefaultCommands(t *testing.T) {
	// given
	c := newChat("test", WithDefaultCommands())

	// when
	result := commandNames(c)

	// then
	expected := []string{"clear", "compact", "effort", "exit", "help", "model"}
	assert.Equal(t, expected, result)
}

type stubSkills struct{ names []string }

func (s stubSkills) Skills() []string { return s.names }

func TestWithSkills(t *testing.T) {
	// given
	c := newChat("test", WithSkills(stubSkills{names: []string{"brainstorm"}}))

	// when
	result := commandNames(c)

	// then
	assert.Contains(t, result, "skills")
}

func TestCommandsAreSortedByName(t *testing.T) {
	// given
	c := newChat("test", WithDefaultCommands())

	// when
	result := commandNames(c)

	// then
	assert.IsIncreasing(t, result)
}

func TestWithCommandOverridesADefault(t *testing.T) {
	// given
	replacement := stubNamedCommand{name: "help"}

	// when
	c := newChat("test", WithCommand(replacement))

	// then
	for _, cmd := range c.Commands() {
		if cmd.Name() == "help" {
			assert.Equal(t, replacement, cmd)
			return
		}
	}
	t.Fatal("help command not registered")
}

type stubNamedCommand struct{ name string }

func (s stubNamedCommand) Name() string                { return s.name }
func (s stubNamedCommand) Help() string                { return "stub" }
func (s stubNamedCommand) Run(command.Context, string) {}
