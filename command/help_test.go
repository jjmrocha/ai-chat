package command

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUsageOf(t *testing.T) {
	tests := []struct {
		name     string
		cmd      Command
		expected string
	}{
		{
			name:     "argumented command with empty spec stays bare",
			cmd:      argumentedStub{stubCommand{name: "mcp"}, ""},
			expected: "/mcp",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			cmd := tc.cmd

			// when
			result := usageOf(cmd)

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestHelpText(t *testing.T) {
	t.Run("entries are sorted by usage", func(t *testing.T) {
		// given
		cmds := []Command{
			stubCommand{name: "model", help: "m"},
			stubCommand{name: "clear", help: "c"},
			stubCommand{name: "exit", help: "e"},
		}

		// when
		result := helpText(cmds)

		// then
		expected := "Commands:\n  /clear c\n  /exit  e\n  /model m"
		assert.Equal(t, expected, result)
	})

	t.Run("usage column is padded to the widest entry", func(t *testing.T) {
		// given
		cmds := []Command{
			argumentedStub{stubCommand{name: "mcp", help: "toggle"}, "[on|off] [name]"},
			stubCommand{name: "help", help: "show"},
		}

		// when
		result := helpText(cmds)

		// then
		lines := strings.Split(result, "\n")
		if assert.Len(t, lines, 3) {
			assert.Equal(t, "  /help                show", lines[1])
			assert.Equal(t, "  /mcp [on|off] [name] toggle", lines[2])
		}
	})

	t.Run("padding counts runes not bytes", func(t *testing.T) {
		// given
		cmds := []Command{
			stubCommand{name: "héllo", help: "a"},
			stubCommand{name: "x", help: "b"},
		}

		// when
		result := helpText(cmds)

		// then
		lines := strings.Split(result, "\n")
		if assert.Len(t, lines, 3) {
			assert.Equal(t, "  /héllo a", lines[1])
			assert.Equal(t, "  /x     b", lines[2])
		}
	})
}

func TestHelpCommand(t *testing.T) {
	t.Run("prints the registry contents as an info line", func(t *testing.T) {
		// given
		reg := &mockedRegistry{
			commandsFunc: func() []Command {
				return []Command{stubCommand{name: "clear", help: "Reset conversation"}}
			},
		}
		ctx := &mockedContext{}

		// when
		Help(reg).Run(ctx, "")

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, infoStream, ctx.printed[0].kind)
			assert.Equal(t, "Commands:\n  /clear Reset conversation", ctx.printed[0].text)
		}
	})

}
