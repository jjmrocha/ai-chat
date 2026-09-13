// Package chat is the headless core of a terminal chat agent. A [Chat] owns the
// conversation transcript and drives an ai-toolkit agent, notifying a single
// [Observer] whenever the transcript changes so a front-end can re-render.
//
// The core has no dependency on any UI toolkit and renders nothing itself: it
// stores plain semantic text in [Line] and leaves every glyph, color and layout
// decision to the front-end. The bundled Bubble Tea renderer lives in package
// ui, but it is only one Observer — drive a Chat from a test, a log sink or
// your own UI just as well.
//
// A Chat is safe for concurrent use. Input submitted while a turn is running is
// queued and replayed in order once the turn ends, so callers never have to
// check whether the core is busy before calling [Chat.Submit].
package chat

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
)

// Line is one entry in the transcript: the text to show and the [command.Kind]
// a front-end styles it by. Text carries no decoration — no prompt glyph, no
// bullet, no indent — so a renderer is free to present it however it likes.
//
// Detail is the second half of a paired entry and is empty for most kinds. For
// [command.Activity] it holds the tool result belonging to the call in Text, so
// a renderer can style request and response differently without parsing.
type Line struct {
	Kind   command.Kind
	Text   string
	Detail string
}

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

var errCancelled = errors.New("cancelled by user")

type agentBackend interface {
	Process(ctx context.Context, input string) (*agent.Response, error)
	ChangeModel(name string) error
	ChangeEffort(e llm.Effort) error
	AvailableModels() []string
	ModelInfo(ctx context.Context) *agent.ModelInfo
	CompactContext(ctx context.Context)
	ResetSession() error
}

// Chat is the headless conversation core. Create one with [New] and hand it to
// a front-end such as ui.Run.
//
// Chat implements [command.Context] and [command.AgentController], so it is the
// value slash commands receive; it also implements agent.Feedback, so the
// ai-toolkit agent reports tool calls and compaction through it. The methods
// serving those two roles are documented as such and are not meant to be called
// directly.
//
// All methods are safe for concurrent use.
type Chat struct {
	name    string
	agent   agentBackend
	baseCtx context.Context

	commands     map[string]command.Command
	telemetryFmt TelemetryFormatter
	statusFmt    StatusFormatter

	mu         sync.Mutex
	transcript []Line
	observer   Observer
	busy       bool
	queue      []string
	cancelTurn context.CancelCauseFunc
	cancelling bool

	pendingTool string
	lastMeta    agent.Metadata
	statusCache *StatusInfo
}

// New creates a Chat named name that drives ag, applying opts in order.
//
// The name appears in the front-end's title bar and may be empty. The /help and
// /exit commands are always registered; every other command comes from an
// option such as [WithDefaultCommands] or [WithCommand], and a later option may
// replace an earlier one by registering the same name.
//
// New registers the Chat as ag's feedback receiver, so a given agent should
// back only one Chat.
func New(name string, ag *agent.Agent, opts ...Option) *Chat {
	c := newChat(name, opts...)
	c.agent = ag
	ag.SetFeedback(c)
	return c
}

func newChat(name string, opts ...Option) *Chat {
	c := &Chat{
		name:         name,
		baseCtx:      context.Background(),
		commands:     map[string]command.Command{},
		telemetryFmt: defaultTelemetryFormatter,
		statusFmt:    defaultStatusFormatter,
	}
	c.register(command.Help(c))
	c.register(command.Exit(c))
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SetContext sets the context used for agent turns and for the model and
// compaction calls the core makes on its own. Cancelling it ends the session's
// work. Call it before the first [Chat.Submit]; ui.Run calls it for you.
func (c *Chat) SetContext(ctx context.Context) { c.baseCtx = ctx }

func (c *Chat) register(cmd command.Command) {
	c.commands[cmd.Name()] = cmd
}

// Name returns the name given to [New], for a front-end to display.
func (c *Chat) Name() string { return c.name }

// Commands returns every registered command, sorted by name. It implements
// [command.Registry] so /help can list them.
func (c *Chat) Commands() []command.Command {
	cmds := slices.Collect(maps.Values(c.commands))
	slices.SortFunc(cmds, func(a, b command.Command) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return cmds
}

// SetObserver installs the observer notified on transcript changes and on
// /exit, replacing any previous one. Passing nil silences both signals.
func (c *Chat) SetObserver(o Observer) {
	c.mu.Lock()
	c.observer = o
	c.mu.Unlock()
}

// Transcript returns a copy of the whole transcript. Front-ends that print
// incrementally should prefer [Chat.TranscriptLen] with [Chat.Since], which
// copies only the part they have not shown yet.
func (c *Chat) Transcript() []Line {
	return c.Since(0)
}

// TranscriptLen returns the number of lines in the transcript. It shrinks to
// zero when [Chat.Clear] resets the session.
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

// Busy reports whether a turn or command is currently running.
func (c *Chat) Busy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.busy
}

// PendingTool returns a description of the tool call in flight, or an empty
// string when none is running. A front-end can show it as progress detail.
func (c *Chat) PendingTool() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.pendingTool
}

// Queued reports whether input submitted during a running turn is still
// waiting to run.
func (c *Chat) Queued() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.queue) > 0
}

// Cancel stops the running agent turn and discards all queued input. The turn
// ends with a "Cancelled." line unless its reply had already arrived. A running
// slash command is not interrupted, and Cancel does nothing when idle.
func (c *Chat) Cancel() {
	c.mu.Lock()
	c.queue = nil
	if c.cancelTurn != nil && !c.cancelling {
		c.cancelling = true
		c.cancelTurn(errCancelled)
	}
	c.mu.Unlock()
	c.notify()
}

// Cancelling reports whether [Chat.Cancel] was called and the running turn has
// not yet returned. A front-end can show it in place of progress detail.
func (c *Chat) Cancelling() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cancelling
}

// LastMetadata returns the metadata of the most recent successful turn. It is
// the zero value before the first turn completes and after [Chat.Clear].
func (c *Chat) LastMetadata() agent.Metadata {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastMeta
}

// Submit queues text as the next input and returns immediately; the work runs
// on its own goroutine.
//
// Text is trimmed, and empty input is ignored. Input beginning with "/" runs as
// a slash command, anything else as an agent turn. Commands and turns share one
// queue and run strictly in submission order, so a command never overlaps a
// turn or another command.
func (c *Chat) Submit(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	queued := c.enqueue(text)
	c.notify()
	if queued {
		return
	}
	go c.process(c.baseCtx, text)
}

func (c *Chat) enqueue(text string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.busy {
		c.queue = append(c.queue, text)
		return true
	}
	c.busy = true
	return false
}

func (c *Chat) dequeue() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.queue) == 0 {
		c.busy = false
		return "", false
	}
	next := c.queue[0]
	c.queue = c.queue[1:]
	return next, true
}

func (c *Chat) resolve(input string) (command.Command, string, bool) {
	name, args, _ := strings.Cut(strings.TrimPrefix(input, "/"), " ")
	args = strings.TrimSpace(args)

	cmd, ok := c.commands[name]
	if !ok {
		c.append(command.Error, "Error: unknown command /"+name)
		return nil, "", false
	}
	return cmd, args, true
}

// Quit asks the observer to end the session. It implements [command.Quitter]
// for /exit and does nothing when no observer is installed.
func (c *Chat) Quit() {
	c.mu.Lock()
	o := c.observer
	c.mu.Unlock()
	if o != nil {
		o.Quit()
	}
}

func (c *Chat) process(ctx context.Context, text string) {
	for ok := true; ok; text, ok = c.dequeue() {
		if strings.HasPrefix(text, "/") {
			if cmd, args, ok := c.resolve(text); ok {
				cmd.Run(c, args)
			}
			continue
		}
		c.turn(ctx, text)
	}
}

func (c *Chat) turn(ctx context.Context, text string) {
	c.append(command.User, text)

	turnCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	c.mu.Lock()
	c.cancelTurn = cancel
	c.mu.Unlock()

	resp, err := c.agent.Process(turnCtx, text)

	c.closeToolCall(formatToolResult("", nil, 0))

	c.mu.Lock()
	c.cancelTurn = nil
	c.cancelling = false
	if err == nil && resp != nil {
		c.lastMeta = resp.Metadata
	}
	c.statusCache = nil
	c.mu.Unlock()

	switch {
	case err != nil && errors.Is(context.Cause(turnCtx), errCancelled):
		c.append(command.Info, "Cancelled.")
	case err != nil:
		c.append(command.Error, "Error: "+err.Error())
	case resp != nil:
		c.append(command.Reply, resp.Content)
		if line := c.telemetryFmt(resp.Metadata); line != "" {
			c.append(command.Telemetry, line)
		}
	default:
		c.append(command.Error, "No response received.")
	}
}

func (c *Chat) append(k command.Kind, text string) {
	c.appendLine(Line{Kind: k, Text: text})
}

func (c *Chat) appendLine(ln Line) {
	c.mu.Lock()
	c.transcript = append(c.transcript, ln)
	c.mu.Unlock()
	c.notify()
}

func (c *Chat) notify() {
	c.mu.Lock()
	o := c.observer
	c.mu.Unlock()
	if o != nil {
		o.TranscriptChanged()
	}
}
