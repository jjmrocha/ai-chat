package chat

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{name: "zero", input: 0, expected: "0"},
		{name: "under 1K", input: 500, expected: "500"},
		{name: "exactly 1K", input: 1000, expected: "1K"},
		{name: "1.5K", input: 1500, expected: "1.50K"},
		{name: "9.99K", input: 9990, expected: "9.99K"},
		{name: "exactly 1M", input: 1000000, expected: "1M"},
		{name: "2.5M", input: 2500000, expected: "2.50M"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			input := tc.input

			// when
			result := formatTokens(input)

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		budget   int
		expected string
	}{
		{name: "under budget is untouched", input: "abc", budget: 5, expected: "abc"},
		{name: "exactly at budget is untouched", input: "abcde", budget: 5, expected: "abcde"},
		{name: "over budget gains an ellipsis", input: "abcdef", budget: 5, expected: "abcde…"},
		{name: "zero budget keeps only the ellipsis", input: "abc", budget: 0, expected: "…"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			input, budget := tc.input, tc.budget

			// when
			result := truncate(input, budget)

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTruncateNeverSplitsARune(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		budget int
	}{
		{name: "cut inside a 2-byte rune", input: strings.Repeat("é", 10), budget: 5},
		{name: "cut inside a 3-byte rune", input: strings.Repeat("€", 10), budget: 5},
		{name: "cut inside a 4-byte emoji", input: strings.Repeat("😀", 10), budget: 5},
		{name: "cut just after a multi-byte rune", input: "é" + strings.Repeat("a", 10), budget: 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			input, budget := tc.input, tc.budget

			// when
			result := truncate(input, budget)

			// then
			assert.True(t, utf8.ValidString(result), "produced invalid UTF-8: %q", result)
			assert.True(t, strings.HasSuffix(result, "…"))
			assert.True(t, strings.HasPrefix(tc.input, strings.TrimSuffix(result, "…")))
		})
	}
}

func TestPlural(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		expected string
	}{
		{name: "zero is plural", n: 0, expected: "0 keys"},
		{name: "one is singular", n: 1, expected: "1 key"},
		{name: "many is plural", n: 5, expected: "5 keys"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// when
			result := plural(tc.n, "key")

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}
