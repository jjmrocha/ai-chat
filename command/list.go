package command

import (
	"strings"

	"github.com/jjmrocha/go-algo/fn"
)

func listText(header string, items []string) string {
	indented := fn.Map(items, func(item string) string { return "  " + item })
	return strings.Join(append([]string{header + ":"}, indented...), "\n")
}

func printList(ctx Context, header, empty string, items []string) {
	if len(items) == 0 {
		ctx.Info(empty)
		return
	}
	ctx.Info(listText(header, items))
}

func printErr(ctx Context, err error) {
	ctx.Error("Error: " + err.Error())
}
