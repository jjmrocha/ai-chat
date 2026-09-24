package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInputHistoryRecall(t *testing.T) {
	t.Run("recall walks backwards then forwards", func(t *testing.T) {
		// given
		m := sized(t, &mockedChatCore{}, 80, 24)
		m.remember("first")
		m.remember("second")

		// when / then
		require.True(t, m.recallOlder())
		assert.Equal(t, "second", m.input.Value())

		require.True(t, m.recallOlder())
		assert.Equal(t, "first", m.input.Value())

		require.True(t, m.recallNewer())
		assert.Equal(t, "second", m.input.Value())
	})

	t.Run("recall past the oldest entry is refused", func(t *testing.T) {
		// given
		m := sized(t, &mockedChatCore{}, 80, 24)
		m.remember("only")
		require.True(t, m.recallOlder())

		// when
		result := m.recallOlder()

		// then
		assert.False(t, result)
	})

	t.Run("empty history has nothing to recall", func(t *testing.T) {
		// given
		m := sized(t, &mockedChatCore{}, 80, 24)

		// when
		result := m.recallOlder()

		// then
		assert.False(t, result)
	})

	t.Run("blank input is not remembered", func(t *testing.T) {
		// given
		m := sized(t, &mockedChatCore{}, 80, 24)

		// when
		m.remember("   ")

		// then
		assert.Empty(t, m.history.entries)
	})

	t.Run("consecutive duplicates are collapsed", func(t *testing.T) {
		// given
		m := sized(t, &mockedChatCore{}, 80, 24)

		// when
		m.remember("same")
		m.remember("same")

		// then
		assert.Len(t, m.history.entries, 1)
	})

	t.Run("returning to the newest restores the draft", func(t *testing.T) {
		// given
		m := sized(t, &mockedChatCore{}, 80, 24)
		m.remember("sent")
		m.setInput("half typed")
		require.True(t, m.recallOlder())
		require.Equal(t, "sent", m.input.Value())

		// when
		result := m.recallNewer()

		// then
		assert.True(t, result)
		assert.Equal(t, "half typed", m.input.Value())
	})
}
