package theme

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestByName(t *testing.T) {
	t.Run("known theme", func(t *testing.T) {
		// given

		// when
		result, ok := ByName("default")

		// then
		assert.True(t, ok)
		assert.Equal(t, Default, result)
	})

	t.Run("case insensitive", func(t *testing.T) {
		// given

		// when
		result, ok := ByName("NORD")

		// then
		assert.True(t, ok)
		assert.Equal(t, Nord, result)
	})

	t.Run("unknown theme", func(t *testing.T) {
		// given

		// when
		_, ok := ByName("unknown")

		// then
		assert.False(t, ok)
	})

	t.Run("empty string", func(t *testing.T) {
		// given

		// when
		_, ok := ByName("")

		// then
		assert.False(t, ok)
	})
}

func TestNames(t *testing.T) {
	// given

	// when
	names := Names()
	sort.Strings(names)

	// then
	expected := []string{"catppuccin", "default", "monokai", "nord"}
	assert.Equal(t, expected, names)
}
