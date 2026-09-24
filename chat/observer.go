package chat

// Observer receives the core's two signals to a front-end. A Chat never renders
// or exits the program itself; it calls these instead.
//
// Both methods may be called from any goroutine, including while the caller is
// inside a Chat method, so an implementation must not block.
type Observer interface {
	// TranscriptChanged reports that the transcript gained a line or was
	// reset, and that the front-end should re-render.
	TranscriptChanged()

	// Quit reports that the session should end, in response to /exit.
	Quit()
}

// SetObserver installs the observer notified on transcript changes and on
// /exit, replacing any previous one. Passing nil silences both signals.
func (c *Chat) SetObserver(o Observer) {
	c.mu.Lock()
	c.observer = o
	c.mu.Unlock()
}

// Quit asks the observer to end the session. It implements [command.Quitter]
// for /exit and does nothing when no observer is installed.
func (c *Chat) Quit() {
	if o := c.currentObserver(); o != nil {
		o.Quit()
	}
}

func (c *Chat) notify() {
	if o := c.currentObserver(); o != nil {
		o.TranscriptChanged()
	}
}

func (c *Chat) currentObserver() Observer {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.observer
}

func (c *Chat) mutate(change func()) {
	c.mu.Lock()
	change()
	c.mu.Unlock()
	c.notify()
}
