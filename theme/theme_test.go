package theme

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaletteNeverUsesAFixedColour(t *testing.T) {
	// A hex value would ignore the profile the user chose, which is the whole
	// point of this palette. Everything here is either the terminal's default
	// text colour (empty) or an ANSI index in 1-15, which the profile defines.
	// The fixed 16-255 cube is as wrong as hex, and 0 is black, which vanishes
	// on a dark background.
	readable := map[string]string{
		"Footer":    Palette.Footer,
		"Activity":  Palette.Activity,
		"Telemetry": Palette.Telemetry,
	}
	hued := map[string]string{
		"HeaderName": Palette.HeaderName,
		"User":       Palette.User,
		"Error":      Palette.Error,
		"Info":       Palette.Info,
		"TurnSep":    Palette.TurnSep,
	}

	for name, value := range readable {
		t.Run(name+" carries text, so it takes the default colour", func(t *testing.T) {
			assert.Empty(t, value)
		})
	}

	for name, value := range hued {
		t.Run(name+" is an index from the profile", func(t *testing.T) {
			// when
			index, err := strconv.Atoi(value)

			// then
			require.NoError(t, err, "%q is not an ANSI index", value)
			assert.GreaterOrEqual(t, index, 1)
			assert.Less(t, index, 16)
		})
	}
}
