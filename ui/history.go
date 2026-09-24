package ui

import "strings"

type history struct {
	entries []string
	idx     int
	draft   string
}

func (h *history) add(text string) {
	h.draft = ""
	if strings.TrimSpace(text) != "" && h.last() != text {
		h.entries = append(h.entries, text)
	}
	h.idx = len(h.entries)
}

func (h *history) last() string {
	if len(h.entries) == 0 {
		return ""
	}
	return h.entries[len(h.entries)-1]
}

func (h *history) older(current string) (string, bool) {
	if h.idx == 0 {
		return "", false
	}
	if h.idx == len(h.entries) {
		h.draft = current
	}
	h.idx--
	return h.entries[h.idx], true
}

func (h *history) newer() (string, bool) {
	if h.idx >= len(h.entries) {
		return "", false
	}
	h.idx++
	if h.idx == len(h.entries) {
		draft := h.draft
		h.draft = ""
		return draft, true
	}
	return h.entries[h.idx], true
}

func (m *model) remember(text string) {
	m.history.add(text)
}

func (m *model) recallOlder() bool {
	if m.input.Line() > 0 {
		return false
	}
	return m.recall(m.history.older(m.input.Value()))
}

func (m *model) recallNewer() bool {
	if m.input.Line() < m.input.LineCount()-1 {
		return false
	}
	return m.recall(m.history.newer())
}

func (m *model) recall(text string, ok bool) bool {
	if ok {
		m.setInput(text)
	}
	return ok
}

func (m *model) setInput(text string) {
	m.input.SetValue(text)
	m.input.CursorEnd()
}
