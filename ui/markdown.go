package ui

import (
	"strconv"

	"charm.land/glamour/v2/ansi"
	glamourstyles "charm.land/glamour/v2/styles"
)

var basicHex = [16]string{
	"#000000", "#7f0000", "#007f00", "#7f7fe0", "#00007f", "#7f007f", "#007f7f", "#e5e5e5",
	"#555555", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff",
}

func markdownStyle(p palette) ansi.StyleConfig {
	// The copy shares pointers with glamour's global: assign fresh ones, never write through them.
	s := glamourstyles.DarkStyleConfig

	s.Document.Color = nil

	s.Heading.Color = colorPtr(p.HeaderName)
	s.H1.Color = colorPtr(p.HeaderName)
	s.H1.BackgroundColor = nil
	s.H6.Color = colorPtr(p.TurnSep)

	s.HorizontalRule.Color = colorPtr(p.TurnSep)

	s.LinkText.Color = colorPtr(p.Info)
	s.LinkText.Bold = nil
	s.LinkText.Underline = boolPtr(true)
	s.Link.Color = colorPtr(p.TurnSep)
	s.Link.Underline = nil

	s.Image.Color = colorPtr(p.Info)
	s.ImageText.Color = colorPtr(p.TurnSep)

	s.Code.Color = colorPtr(p.Info)
	s.Code.BackgroundColor = nil

	s.CodeBlock.Color = colorPtr(p.TurnSep)
	s.CodeBlock.Chroma = &ansi.Chroma{
		Error:               chromaColor(p.Error),
		Comment:             chromaColor(p.TurnSep),
		CommentPreproc:      chromaColor(p.TurnSep),
		Keyword:             chromaColor(p.HeaderName),
		KeywordReserved:     chromaColor(p.HeaderName),
		KeywordNamespace:    chromaColor(p.HeaderName),
		KeywordType:         chromaColor(p.HeaderName),
		LiteralNumber:       chromaColor(p.Info),
		LiteralString:       chromaColor(p.Info),
		LiteralStringEscape: chromaColor(p.Info),
	}

	return s
}

func chromaHex(index string) string {
	i, err := strconv.Atoi(index)
	if err != nil || i < 0 || i >= len(basicHex) {
		return ""
	}
	return basicHex[i]
}

func chromaColor(index string) ansi.StylePrimitive {
	return ansi.StylePrimitive{Color: colorPtr(chromaHex(index))}
}

func colorPtr(c string) *string {
	if c == "" {
		return nil
	}
	return &c
}

func boolPtr(b bool) *bool {
	return &b
}
