package command

import (
	"strings"

	"github.com/jjmrocha/ai-chat/theme"
)

type themeCmd struct{}

// Theme returns the /theme command: show the available themes or switch to one.
func Theme() Command {
	return themeCmd{}
}

func (themeCmd) Name() string {
	return "theme"
}

func (themeCmd) Help() string {
	return "Show or switch theme"
}

func (themeCmd) Args() string {
	return "[name]"
}

func (themeCmd) Run(ctx Context, args string) {
	if args == "" {
		ctx.Print(Info, listText("Themes", theme.Names()))
		return
	}
	if err := ctx.ChangeTheme(args); err != nil {
		ctx.Print(Error, "Unknown theme, available: "+strings.Join(theme.Names(), ", "))
		return
	}
	ctx.Print(Info, "Theme: "+args)
}
