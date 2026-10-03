package ui

import (
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/go-algo/fn"
)

func (m model) emit() (tea.Model, tea.Cmd) {
	if m.printing {
		return m, nil
	}

	blocks, next := m.pending()
	if len(blocks) == 0 {
		return m, nil
	}
	m.cursor = next
	m.printing = true

	return m, tea.Sequence(m.printCmds(blocks)...)
}

// printBacklog writes the transcript the core already holds straight to w,
// before the program starts. Printing it through tea.Println instead would
// misplace it: until the first frame is flushed, the renderer still believes
// the live region spans the whole terminal.
func (m model) printBacklog(w io.Writer, width, height int) model {
	if width <= 0 || height <= 0 {
		return m
	}

	m.setSize(width, height)
	blocks, next := m.pending()
	for _, b := range blocks {
		_, _ = fmt.Fprintln(w, b)
	}
	m.cursor = next

	return m
}

func (m model) printCmds(blocks []string) []tea.Cmd {
	limit := m.printLimit()
	cmds := make([]tea.Cmd, 0, len(blocks)+1)
	for _, b := range blocks {
		for _, chunk := range chunkBlock(b, limit) {
			cmds = append(cmds, tea.Println(chunk))
		}
	}
	return append(cmds, func() tea.Msg { return printedMsg{} })
}

// printWidth is one column short of the terminal so that no printed line ever
// fills it exactly: bubbletea's insertAbove counts a full-width line as two
// rows, scrolls one line too far and then paints over live content.
func (m model) printWidth() int { return max(m.width-1, 0) }

func (m model) printLimit() int {
	if m.height == 0 {
		return 0
	}

	return max(m.height-lipgloss.Height(m.liveRegion()), 1)
}

func chunkBlock(block string, limit int) []string {
	lines := strings.Split(block, "\n")
	if limit <= 0 || len(lines) <= limit {
		return []string{block}
	}

	chunks := make([]string, 0, (len(lines)+limit-1)/limit)
	for len(lines) > limit {
		chunks = append(chunks, strings.Join(lines[:limit], "\n"))
		lines = lines[limit:]
	}

	return append(chunks, strings.Join(lines, "\n"))
}

func (m model) pending() ([]string, chat.Cursor) {
	lines, next := m.core.Next(m.cursor)
	return fn.Map(lines, func(ln chat.Line) string {
		return "\n" + ansi.Wrap(m.renderBlock(ln), m.printWidth(), "")
	}), next
}
