package chat

import "slices"

// Cursor marks how much of the transcript a front-end has already shown. The
// zero value is the start of the transcript; pass each Cursor [Chat.Next]
// returns back to the following call.
type Cursor struct {
	epoch uint64
	n     int
}

// Next returns a copy of the lines added since cur, and the cursor to pass next
// time. If /clear reset the transcript after cur was taken, it returns
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
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.transcript)
}

func (c *Chat) append(k Kind, text string) {
	c.appendLine(Line{Kind: k, Text: text})
}

func (c *Chat) appendLine(ln Line) {
	ln.Text = sanitize(ln.Text)
	ln.Detail = sanitize(ln.Detail)
	c.mutate(func() { c.transcript = append(c.transcript, ln) })
}
