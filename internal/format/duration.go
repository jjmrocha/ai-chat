// Package format holds the text formatting shared by the chat core and the
// bundled front-end.
package format

import (
	"strconv"
	"time"
)

// Duration renders d the way the transcript and the front-end's progress line
// both show elapsed time: whole seconds and above in Go's own notation
// ("2m2s"), shorter spans in milliseconds ("340ms", "<1ms").
func Duration(d time.Duration) string {
	switch {
	case d >= time.Second:
		return d.Truncate(time.Second).String()
	case d >= time.Millisecond:
		return strconv.FormatInt(d.Milliseconds(), 10) + "ms"
	default:
		return "<1ms"
	}
}
