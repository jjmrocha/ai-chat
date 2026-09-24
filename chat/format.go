package chat

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func formatTokens(tokens int) string {
	switch {
	case tokens >= 1_000_000:
		return scaleSuffix(tokens, 1_000_000, "M")
	case tokens >= 1_000:
		return scaleSuffix(tokens, 1_000, "K")
	default:
		return strconv.Itoa(tokens)
	}
}

func scaleSuffix(n, unit int, suffix string) string {
	v := float64(n) / float64(unit)
	if v == float64(int(v)) {
		return strconv.Itoa(n/unit) + suffix
	}
	return fmt.Sprintf("%.2f%s", v, suffix)
}

func formatBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("<%.1f MB>", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("<%.1f KB>", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("<%d B>", n)
	}
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}

	return strconv.Itoa(n) + " " + noun + "s"
}

func isControl(r rune) bool {
	return r < 0x20 || (r >= 0x7f && r <= 0x9f)
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && isControl(r) {
			return -1
		}

		return r
	}, ansi.Strip(s))
}

func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if isControl(r) {
			return -1
		}

		return r
	}, s)
}

func firstLine(s string) string {
	head, _, _ := strings.Cut(s, "\n")

	return head
}

func truncate(s string, budget int) string {
	if len(s) <= budget {
		return s
	}

	cut := budget
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}

	return s[:cut] + "…"
}
