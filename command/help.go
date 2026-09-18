package command

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jjmrocha/go-algo/fn"
)

// Registry is the set of commands /help lists. chat.Chat implements it, so
// [Help] is registered for you and this is only of interest to a front-end
// building its own command list.
type Registry interface {
	// Commands returns every registered command, sorted by name.
	Commands() []Command
}

type helpCmd struct{ reg Registry }

// Help returns the /help command, which lists every command in reg with its
// usage and description, aligned in two columns. It is registered
// automatically; pass your own through chat.WithCommand to replace it.
func Help(reg Registry) Command {
	return helpCmd{reg: reg}
}

func (helpCmd) Name() string {
	return "help"
}

func (helpCmd) Help() string {
	return "Show this message"
}

func (c helpCmd) Run(ctx Context, _ string) {
	ctx.Print(Info, helpText(c.reg.Commands()))
}

func helpText(cmds []Command) string {
	type entry struct{ usage, desc string }

	entries := fn.Map(cmds, func(cmd Command) entry {
		return entry{usage: usageOf(cmd), desc: cmd.Help()}
	})
	slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.usage, b.usage) })

	width := fn.Fold(entries, 0, func(w int, e entry) int {
		return max(w, utf8.RuneCountInString(e.usage))
	})

	lines := fn.Map(entries, func(e entry) string {
		pad := strings.Repeat(" ", width-utf8.RuneCountInString(e.usage))
		return "  " + e.usage + pad + " " + e.desc
	})
	return strings.Join(append([]string{"Commands:"}, lines...), "\n")
}

func usageOf(cmd Command) string {
	usage := "/" + cmd.Name()
	if a, ok := cmd.(Argumented); ok && a.Args() != "" {
		usage += " " + a.Args()
	}
	return usage
}
