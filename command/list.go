package command

import "strings"

func listText(header string, items []string) string {
	lines := make([]string, 0, len(items)+1)
	lines = append(lines, header+":")
	for _, item := range items {
		lines = append(lines, "  "+item)
	}
	return strings.Join(lines, "\n")
}

func printList(ctx Context, header, empty string, items []string) {
	if len(items) == 0 {
		ctx.Print(Info, empty)
		return
	}
	ctx.Print(Info, listText(header, items))
}

func printErr(ctx Context, err error) {
	ctx.Print(Error, "Error: "+err.Error())
}
