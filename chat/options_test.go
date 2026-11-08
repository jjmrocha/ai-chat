package chat

import (
	"context"
	"fmt"
	"testing"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/ai-toolkit/mcp"
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
	result := commandNames(newChat("test", &mockedModelBackend{}))

	// then
	assert.Equal(t, []string{"exit", "help"}, result)
}

func TestWithDefaultCommands(t *testing.T) {
	// given
	c := newChat("test", &mockedModelBackend{}, WithDefaultCommands())

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
	c := newChat("test", &mockedModelBackend{}, WithSkills(stubSkills{names: []string{"brainstorm"}}))

	// when
	result := commandNames(c)

	// then
	assert.Contains(t, result, "skills")
}

func TestSkillCommandHelpIsItsDescription(t *testing.T) {
	// given
	c := newChat("test", &mockedModelBackend{}, WithSkillCommand("brainstorm", "Explore an idea"))

	// when
	var result string
	for _, cmd := range c.Commands() {
		if cmd.Name() == "brainstorm" {
			result = cmd.Help()
		}
	}

	// then
	assert.Equal(t, "Explore an idea", result)
}

func TestWithCommandReplacesASkillCommand(t *testing.T) {
	// given
	var args argsCommand
	args.name = "brainstorm"
	backend := &mockedAgentBackend{}
	c, _ := newTestChat(t, backend, WithSkillCommand("brainstorm", "Explore"), WithCommand(&args))

	// when
	c.Submit("/brainstorm idea")
	c.Wait()

	// then
	assert.Empty(t, backend.inputs())
	assert.Equal(t, "idea", args.args)
}

func TestWithCommandOverridesADefault(t *testing.T) {
	// given
	replacement := stubNamedCommand{name: "help"}

	// when
	c := newChat("test", &mockedModelBackend{}, WithCommand(replacement))

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

func TestWithTelemetryFormatter(t *testing.T) {
	tests := []struct {
		name      string
		formatter TelemetryFormatter
		expected  []Line
	}{
		{
			name:      "custom text becomes the telemetry line",
			formatter: func(meta agent.Metadata) string { return fmt.Sprintf("%d calls", meta.ToolCalls) },
			expected: []Line{
				{Kind: User, Text: "hi"},
				{Kind: Reply, Text: "reply"},
				{Kind: Telemetry, Text: "2 calls"},
			},
		},
		{
			name:      "empty text suppresses the line",
			formatter: func(agent.Metadata) string { return "" },
			expected: []Line{
				{Kind: User, Text: "hi"},
				{Kind: Reply, Text: "reply"},
			},
		},
		{
			name:      "nil keeps the default formatter",
			formatter: nil,
			expected: []Line{
				{Kind: User, Text: "hi"},
				{Kind: Reply, Text: "reply"},
				{Kind: Telemetry, Text: " 2 tool calls"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			backend := &mockedAgentBackend{
				processFunc: func(context.Context, string) (*agent.Response, error) {
					return &agent.Response{Content: "reply", Metadata: agent.Metadata{ToolCalls: 2}}, nil
				},
			}
			c, _ := newTestChat(t, backend, WithTelemetryFormatter(tc.formatter))

			// when
			c.Submit("hi")
			c.Wait()

			// then
			assert.Equal(t, tc.expected, transcript(c))
		})
	}
}

func TestWithStatusFormatter(t *testing.T) {
	tests := []struct {
		name      string
		formatter StatusFormatter
		expected  string
	}{
		{
			name:      "custom formatter renders the status line",
			formatter: func(info StatusInfo) string { return "model " + info.Model },
			expected:  "model m1",
		},
		{
			name:      "nil keeps the default formatter",
			formatter: nil,
			expected:  "m1 · ctx: 0% · tokens: 0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			model := &mockedModelBackend{info: llm.ModelInfo{Name: "m1"}}
			c, _ := newTestChatWithModel(t, &mockedAgentBackend{}, model, WithStatusFormatter(tc.formatter))

			// when
			result := c.StatusText()

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestWithMCP(t *testing.T) {
	// given
	c := newChat("test", &mockedModelBackend{}, WithMCP(stubMCP{}))

	// when
	result := commandNames(c)

	// then
	assert.Contains(t, result, "mcp")
}

type stubMCP struct{}

func (stubMCP) Status() []mcp.Status                { return nil }
func (stubMCP) Start(context.Context, string) error { return nil }
func (stubMCP) Stop(string) error                   { return nil }
