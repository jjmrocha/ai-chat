package chat

import (
	"slices"

	"github.com/jjmrocha/ai-chat/command"
)

// Cursor marks how much of the transcript a front-end has already shown. The
// zero value is the start of the transcript; pass each Cursor [Chat.Next]
// returns back to the following call.
type Cursor struct {
	epoch uint64
	n     int
}

// Next returns a copy of the lines added since cur, and the cursor to pass next
// time. If [Chat.Clear] reset the transcript after cur was taken, it returns
// the new transcript from its first line, so a front-end printing incrementally
// never skips or repeats a line.
func (c *Chat) Next(cur Cursor) ([]Line, Cursor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cur.epoch != c.epoch || cur.n > len(c.transcript) {
		cur = Cursor{epoch: c.epoch}
	}
	lines := slices.Clone(c.transcript[cur.n:])
	return lines, Cursor{epoch: c.epoch, n: len(c.transcript)}
}

// Transcript returns a copy of the whole transcript. Front-ends that print
// incrementally should prefer [Chat.Next], which copies only the part they have
// not shown yet.
func (c *Chat) Transcript() []Line {
	return c.Since(0)
}

// TranscriptLen returns the number of lines in the transcript. It shrinks to
// zero when [Chat.Clear] resets the session. A length alone cannot tell a reset
// that has since regrown from new lines; use [Chat.Next] to follow the
// transcript.
func (c *Chat) TranscriptLen() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.transcript)
}

// Since returns a copy of the transcript from line n onward, for a front-end
// that has already shown the first n lines. It returns nil when n is negative
// or past the end, which is what a caller sees after [Chat.Clear] has reset the
// transcript beneath it.
func (c *Chat) Since(n int) []Line {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n < 0 || n > len(c.transcript) {
		return nil
	}
	return slices.Clone(c.transcript[n:])
}

func (c *Chat) append(k command.Kind, text string) {
	c.appendLine(Line{Kind: k, Text: text})
}

func (c *Chat) appendLine(ln Line) {
	ln.Text = sanitize(ln.Text)
	ln.Detail = sanitize(ln.Detail)
	c.mutate(func() { c.transcript = append(c.transcript, ln) })
}
