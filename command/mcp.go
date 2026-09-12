package command

import (
	"context"
	"strings"
)

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
		statuses := c.mgr.Status()
		items := make([]string, 0, len(statuses))
		for _, s := range statuses {
			state := "off"
			if s.Active {
				state = "on"
			}
			items = append(items, s.Name+": "+state)
		}
		printList(ctx, "MCP servers", "No MCP servers registered.", items)

	case "on", "off":
		target, ok := c.resolveName(name)
		if !ok {
			ctx.Print(Error, "Specify an MCP name: /mcp "+action+" <name>")
			return
		}
		var err error
		verb := "started"
		if action == "on" {
			err = c.mgr.Start(context.Background(), target)
		} else {
			err = c.mgr.Stop(target)
			verb = "stopped"
		}
		if err != nil {
			printErr(ctx, err)
			return
		}
		ctx.Print(Info, "MCP "+target+" "+verb+".")

	default:
		ctx.Print(Error, "Usage: /mcp [on|off] [name]")
	}
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
