package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jjmrocha/ai-chat/chat"
	"github.com/jjmrocha/ai-chat/command"
)

func TestRenderBlockAppliesGlyphs(t *testing.T) {
	tests := []struct {
		name     string
		line     chat.Line
		contains []string
		absent   []string
	}{
		{
			name:     "user line gains the prompt glyph",
			line:     chat.Line{Kind: command.User, Text: "hello"},
			contains: []string{"❯ hello"},
		},
		{
			name:     "activity with detail gains both glyphs",
			line:     chat.Line{Kind: command.Activity, Text: "read(x)", Detail: "ok"},
			contains: []string{"● read(x)", "⎿ ok"},
		},
		{
			name:     "activity without detail omits the detail glyph",
			line:     chat.Line{Kind: command.Activity, Text: "read(x)"},
			contains: []string{"● read(x)"},
			absent:   []string{"⎿"},
		},
		{
			name:     "info line is unadorned",
			line:     chat.Line{Kind: command.Info, Text: "note"},
			contains: []string{"note"},
			absent:   []string{"❯", "●", "⎿"},
		},
		{
			name:     "error line is unadorned",
			line:     chat.Line{Kind: command.Error, Text: "boom"},
			contains: []string{"boom"},
			absent:   []string{"❯", "●"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			m := sized(t, &mockedChatCore{}, 80, 24)

			// when
			result := m.renderBlock(tc.line)

			// then
			for _, want := range tc.contains {
				assert.Contains(t, result, want)
			}
			for _, unwanted := range tc.absent {
				assert.NotContains(t, result, unwanted)
			}
		})
	}
}

func TestRenderBlockStylesMarkdownReplies(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		contains    []string
		absent      []string
		sgrParams   []string
		noSgrParams []string
	}{
		{
			name:        "inline emphasis renders as attributes, not markup",
			text:        "**bold** *italic* ~~gone~~",
			contains:    []string{"bold", "italic", "gone"},
			absent:      []string{"**", "~~", "*italic*"},
			sgrParams:   []string{"1", "3", "9"},
			noSgrParams: []string{"38", "48"},
		},
		{
			name:        "inline code drops the backticks and uses the info color",
			text:        "run `make test` now",
			contains:    []string{"make test"},
			absent:      []string{"`"},
			sgrParams:   []string{"36"},
			noSgrParams: []string{"38", "48"},
		},
		{
			name:        "link keeps text and URL, text underlined in the info color",
			text:        "see [docs](https://example.com)",
			contains:    []string{"docs", "https://example.com"},
			absent:      []string{"](", "[docs"},
			sgrParams:   []string{"4", "36", "90"},
			noSgrParams: []string{"38", "48"},
		},
		{
			name:        "fenced code is highlighted with basic colors only",
			text:        "```go\nfunc main() {} // hi\n```",
			contains:    []string{"func", "// hi"},
			absent:      []string{"```"},
			sgrParams:   []string{"31", "90"},
			noSgrParams: []string{"38", "48"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			m := sized(t, &mockedChatCore{}, 80, 24)

			// when
			result := m.renderBlock(chat.Line{Kind: command.Reply, Text: tc.text})

			// then
			params := sgrParams(result)
			for _, want := range tc.contains {
				assert.Contains(t, stripSGR(result), want)
			}
			for _, unwanted := range tc.absent {
				assert.NotContains(t, stripSGR(result), unwanted)
			}
			for _, want := range tc.sgrParams {
				assert.Truef(t, params[want], "SGR parameter %s missing in %q", want, result)
			}
			for _, unwanted := range tc.noSgrParams {
				assert.Falsef(t, params[unwanted], "SGR parameter %s present in %q", unwanted, result)
			}
		})
	}
}

var sgrPattern = regexp.MustCompile("\x1b\\[([0-9;]*)m")

func sgrParams(s string) map[string]bool {
	params := map[string]bool{}
	for _, match := range sgrPattern.FindAllStringSubmatch(s, -1) {
		for _, p := range strings.Split(match[1], ";") {
			params[p] = true
		}
	}
	return params
}

func stripSGR(s string) string {
	return sgrPattern.ReplaceAllString(s, "")
}
