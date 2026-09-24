package command

import (
	"context"
	"strings"
	"time"

	"github.com/jjmrocha/ai-toolkit/mcp"
	"github.com/jjmrocha/go-algo/fn"
)

// MCPController is the MCP server manager /mcp drives. Pass one to [MCP], or
// to chat.WithMCP, which wires it for you.
type MCPController interface {
	// Status lists every registered server and whether it is running.
	Status() []mcp.Status

	// Start launches the named server.
	Start(ctx context.Context, name string) error

	// Stop shuts the named server down.
	Stop(name string) error
}

const mcpStartTimeout = 2 * time.Minute

type mcpCmd struct{ mgr MCPController }

// MCP returns the /mcp command, which lists the servers mgr knows about and
// starts or stops them by name. With exactly one server registered, the name
// may be omitted.
func MCP(mgr MCPController) Command {
	return mcpCmd{mgr: mgr}
}

func (mcpCmd) Name() string {
	return "mcp"
}

func (mcpCmd) Help() string {
	return "Show or toggle MCP servers"
}

func (mcpCmd) Args() string {
	return "[on|off] [name]"
}

func (c mcpCmd) Run(ctx Context, args string) {
	action, name, _ := strings.Cut(args, " ")
	action = strings.TrimSpace(action)
	name = strings.TrimSpace(name)

	switch action {
	case "":
		c.list(ctx)
	case "on", "off":
		c.toggle(ctx, action, name)
	default:
		ctx.Print(Error, "Usage: /mcp [on|off] [name]")
	}
}

func (c mcpCmd) list(ctx Context) {
	items := fn.Map(c.mgr.Status(), func(s mcp.Status) string {
		state := "off"
		if s.Active {
			state = "on"
		}
		return s.Name + ": " + state
	})
	printList(ctx, "MCP servers", "No MCP servers registered.", items)
}

func (c mcpCmd) toggle(ctx Context, action, name string) {
	target, ok := c.resolveName(name)
	if !ok {
		ctx.Print(Error, "Specify an MCP name: /mcp "+action+" <name>")
		return
	}

	verb, err := c.switchServer(ctx.Context(), action == "on", target)
	if err != nil {
		printErr(ctx, err)
		return
	}
	ctx.Print(Info, "MCP "+target+" "+verb+".")
}

func (c mcpCmd) switchServer(ctx context.Context, on bool, name string) (verb string, err error) {
	if on {
		ctx, cancel := context.WithTimeout(ctx, mcpStartTimeout)
		defer cancel()
		return "started", c.mgr.Start(ctx, name)
	}
	return "stopped", c.mgr.Stop(name)
}

func (c mcpCmd) resolveName(name string) (string, bool) {
	if name != "" {
		return name, true
	}
	if statuses := c.mgr.Status(); len(statuses) == 1 {
		return statuses[0].Name, true
	}
	return "", false
}
