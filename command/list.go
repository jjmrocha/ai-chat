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
