package format

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDuration(t *testing.T) {
	testCases := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{name: "under a millisecond", input: 400 * time.Microsecond, expected: "<1ms"},
		{name: "milliseconds", input: 340 * time.Millisecond, expected: "340ms"},
		{name: "exactly a second", input: time.Second, expected: "1s"},
		{name: "drops the fraction of a second", input: 1700 * time.Millisecond, expected: "1s"},
		{name: "minutes and seconds", input: 122 * time.Second, expected: "2m2s"},
		{name: "whole minutes", input: 2 * time.Minute, expected: "2m0s"},
		{name: "hours", input: time.Hour + 61*time.Second, expected: "1h1m1s"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			input := tc.input

			// when
			result := Duration(input)

			// then
			assert.Equal(t, tc.expected, result)
		})
	}
}
