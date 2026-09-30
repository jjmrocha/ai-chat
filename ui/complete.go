package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jjmrocha/go-algo/fn"

	"github.com/jjmrocha/ai-chat/command"
)

const maxCompletionRows = 3

type completion struct {
	name string
	help string
}

type completer struct {
	all     []completion
	matches []completion
	sel     int
	offset  int
	input   string
	closed  bool
}

func newCompleter(cmds []command.Command) completer {
	return completer{
		all: fn.Map(cmds, func(cmd command.Command) completion {
			return completion{name: cmd.Name(), help: cmd.Help()}
		}),
	}
}

func (c *completer) update(input string) {
	if input == c.input {
		return
	}
	c.input = input
	c.closed = false
	c.sel = 0
	c.offset = 0
	c.matches = nil

	prefix, ok := strings.CutPrefix(input, "/")
	if !ok || strings.ContainsAny(prefix, " \n") {
		return
	}
	prefix = strings.ToLower(prefix)
	c.matches = fn.Filter(c.all, func(cmd completion) bool {
		return strings.HasPrefix(strings.ToLower(cmd.name), prefix)
	})
}

func (c *completer) open() bool { return !c.closed && len(c.matches) > 0 }

func (c *completer) close() { c.closed = true }

func (c *completer) selected() string { return c.matches[c.sel].name }

func (c *completer) next() {
	if c.sel < len(c.matches)-1 {
		c.sel++
	}
	if c.sel >= c.offset+maxCompletionRows {
		c.offset = c.sel - maxCompletionRows + 1
	}
}

func (c *completer) prev() {
	if c.sel > 0 {
		c.sel--
	}
	if c.sel < c.offset {
		c.offset = c.sel
	}
}

func (c completer) visible() ([]completion, int) {
	end := min(c.offset+maxCompletionRows, len(c.matches))
	return c.matches[c.offset:end], c.sel - c.offset
}

func (m *model) handleCompletionKey(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "up":
		m.completer.prev()
	case "down":
		m.completer.next()
	case "tab":
		m.setInput("/" + m.completer.selected() + " ")
		m.completer.update(m.input.Value())
	case "enter":
		text := "/" + m.completer.selected()
		m.completer.close()
		m.submit(text)
	case "esc":
		m.completer.close()
	default:
		return false
	}
	return true
}

func (m model) completionView() string {
	rows, sel := m.completer.visible()
	width := 0
	for _, row := range rows {
		width = max(width, len(row.name)+1)
	}

	lines := make([]string, maxCompletionRows-len(rows), maxCompletionRows)
	for i, row := range rows {
		marker, style := "  ", m.styles.completion
		if i == sel {
			marker, style = "› ", m.styles.selected
		}
		text := fmt.Sprintf("%s%-*s  %s", marker, width, "/"+row.name, row.help)
		lines = append(lines, ansi.Truncate(style.Render(text), m.width, "…"))
	}
	return strings.Join(lines, "\n")
}
