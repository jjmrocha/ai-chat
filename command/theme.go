package command

import (
	"sort"
	"strings"

	"github.com/jjmrocha/ai-chat/theme"
)

type themeCmd struct{}

func Theme() Command          { return themeCmd{} }
func (themeCmd) Name() string { return "theme" }
func (themeCmd) Help() string { return "/theme [name]    Show or switch theme" }

func (themeCmd) Run(ctx Context, args string) {
	if args == "" {
		names := theme.Names()
		sort.Strings(names)
		ctx.Print(Info, "Theme: "+strings.Join(names, ", "))
		return
	}
	if err := ctx.ChangeTheme(args); err != nil {
		names := theme.Names()
		sort.Strings(names)
		ctx.Print(Error, "Unknown theme, available: "+strings.Join(names, ", "))
		return
	}
	ctx.Print(Info, "Theme: "+args)
}
