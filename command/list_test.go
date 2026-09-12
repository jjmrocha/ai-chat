package command

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListText(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		items    []string
		expected string
	}{
		{
			name:     "no items renders only the header",
			header:   "Models",
			items:    nil,
			expected: "Models:",
		},
		{
			name:     "single item is indented",
			header:   "Models",
			items:    []string{"a"},
			expected: "Models:\n  a",
		},
		{
			name:     "items keep their given order",
			header:   "Skills",
			items:    []string{"b", "a"},
			expected: "Skills:\n  b\n  a",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			header, items := tc.header, tc.items

			// when
			result := listText(header, items)

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPrintList(t *testing.T) {
	t.Run("empty items print the empty message", func(t *testing.T) {
		// given
		ctx := &mockedContext{}

		// when
		printList(ctx, "Models", "No models available.", nil)

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Info, ctx.printed[0].kind)
			assert.Equal(t, "No models available.", ctx.printed[0].text)
		}
	})

	t.Run("non-empty items print the list", func(t *testing.T) {
		// given
		ctx := &mockedContext{}

		// when
		printList(ctx, "Models", "No models available.", []string{"a", "b"})

		// then
		if assert.Len(t, ctx.printed, 1) {
			assert.Equal(t, Info, ctx.printed[0].kind)
			assert.Equal(t, "Models:\n  a\n  b", ctx.printed[0].text)
		}
	})
}

func TestPrintErr(t *testing.T) {
	// given
	ctx := &mockedContext{}

	// when
	printErr(ctx, errors.New("boom"))

	// then
	if assert.Len(t, ctx.printed, 1) {
		assert.Equal(t, Error, ctx.printed[0].kind)
		assert.Equal(t, "Error: boom", ctx.printed[0].text)
	}
}
