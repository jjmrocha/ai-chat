package command

import (
	"context"
	"fmt"
	"strings"
)

type mcpCmd struct{ mgr MCPController }

// MCP returns the /mcp command bound to mgr: show or toggle MCP servers.
func MCP(mgr MCPController) Command { return mcpCmd{mgr: mgr} }
func (mcpCmd) Name() string         { return "mcp" }
func (mcpCmd) Help() string         { return "/mcp [on|off] [name]  Show or toggle MCP servers" }
func (c mcpCmd) Run(ctx Context, args string) {
	action, name, _ := strings.Cut(args, " ")
	action = strings.TrimSpace(action)
	name = strings.TrimSpace(name)

	switch action {
	case "":
		statuses := c.mgr.GetStatus()
		if len(statuses) == 0 {
			ctx.Print(Info, "No MCP servers registered.")
			return
		}
		lines := make([]string, 0, len(statuses)+1)
		lines = append(lines, "MCP servers:")
		for _, s := range statuses {
			state := "off"
			if s.Active {
				state = "on"
			}
			lines = append(lines, fmt.Sprintf("  %s: %s", s.Name, state))
		}
		ctx.Print(Info, strings.Join(lines, "\n"))

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
			ctx.Print(Error, "Error: "+err.Error())
			return
		}
		ctx.Print(Info, "MCP "+target+" "+verb+".")

	default:
		ctx.Print(Error, "Usage: /mcp [on|off] [name]")
	}
}

// resolveName returns name, or the sole registered server's name when name is
// empty and exactly one server is registered.
func (c mcpCmd) resolveName(name string) (string, bool) {
	if name != "" {
		return name, true
	}
	if statuses := c.mgr.GetStatus(); len(statuses) == 1 {
		return statuses[0].Name, true
	}
	return "", false
}
