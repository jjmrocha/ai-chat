package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
)

type (
	waitingTextMsg    string
	waitingPrintMsg   string
	waitingPrintedMsg struct{}
	waitingStopMsg    struct{}
)

// Waiting is a one-line spinner with a message, shown while an app does slow
// work outside the chat, such as starting up. Create it with [StartWaiting].
type Waiting struct {
	program *tea.Program
	out     io.Writer
	done    chan struct{}

	mu      sync.Mutex
	stopped bool
}

// StartWaiting shows a spinner followed by text on the terminal and returns at
// once. The line looks like the chat's thinking line and is cut at the
// terminal width. Keys pressed while it is shown are discarded, except Ctrl+C:
// it erases the line and then sends the process an interrupt signal, as Ctrl+C
// would without it.
//
// Call [Waiting.Stop] before printing anything else or calling [Run]. When ctx
// is cancelled, the line is erased as if Stop had been called. When standard
// output is not a terminal, nothing is drawn and the methods do nothing.
func StartWaiting(ctx context.Context, text string) *Waiting {
	fd := os.Stdout.Fd()
	width, height, _ := term.GetSize(fd)
	return startWaiting(ctx, text, os.Stdout, term.IsTerminal(fd), width, interruptProcess,
		tea.WithWindowSize(width, height),
	)
}

func startWaiting(ctx context.Context, text string, out io.Writer, terminal bool, width int, interrupt func(), opts ...tea.ProgramOption) *Waiting {
	if !terminal {
		return &Waiting{out: out}
	}

	opts = append(opts, tea.WithOutput(out), tea.WithoutSignalHandler())
	p := tea.NewProgram(newWaitingModel(text, width), opts...)
	w := &Waiting{program: p, out: out, done: make(chan struct{})}
	go func() {
		final, _ := p.Run()
		close(w.done)
		if m, ok := final.(waitingModel); ok && m.interrupted {
			interrupt()
		}
	}()
	go w.stopOn(ctx)
	return w
}

func interruptProcess() {
	if p, err := os.FindProcess(os.Getpid()); err == nil {
		_ = p.Signal(os.Interrupt)
	}
}

func (w *Waiting) stopOn(ctx context.Context) {
	select {
	case <-ctx.Done():
		w.Stop()
	case <-w.done:
	}
}

// SetText replaces the message beside the spinner. It does nothing after
// [Waiting.Stop].
func (w *Waiting) SetText(text string) {
	if w.program == nil {
		return
	}
	w.program.Send(waitingTextMsg(text))
}

// Println prints text as a line that stays above the spinner, which keeps
// going below it. Text with newlines prints as several lines, and empty text
// prints nothing. When nothing is drawn, because standard output is not a
// terminal or [Waiting.Stop] was called, text is written to standard output.
func (w *Waiting) Println(text string) {
	if text == "" {
		return
	}
	if w.printAboveSpinner(text) {
		return
	}

	if w.program != nil {
		<-w.done
	}
	_, _ = fmt.Fprintln(w.out, text)
}

func (w *Waiting) printAboveSpinner(text string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.program == nil || w.stopped {
		return false
	}
	w.program.Send(waitingPrintMsg(text))
	return true
}

// Stop erases the line and returns once it is gone, so the next output starts
// on a clean line. It is safe to call more than once.
func (w *Waiting) Stop() {
	if w.program == nil {
		return
	}

	w.mu.Lock()
	w.stopped = true
	w.mu.Unlock()

	w.program.Send(waitingStopMsg{})
	<-w.done
}

type waitingModel struct {
	spinner spinner.Model
	style   lipgloss.Style
	text    string
	width   int

	ticks    int
	queue    []string
	printing bool
	stopping bool

	stopped     bool
	interrupted bool
}

func newWaitingModel(text string, width int) waitingModel {
	return waitingModel{
		spinner: newSpinner(),
		style:   newStyles().footer,
		text:    text,
		width:   width,
	}
}

func (m waitingModel) Init() tea.Cmd { return m.spinner.Tick }

func (m waitingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case waitingTextMsg:
		m.text = string(msg)
	case waitingPrintMsg:
		m.queue = append(m.queue, string(msg))
		return m.flush()
	case waitingPrintedMsg:
		m.printing = false
		return m.flush()
	case waitingStopMsg:
		m.stopping = true
		return m.flush()
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.stopped, m.interrupted = true, true
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var tick tea.Cmd
		m.spinner, tick = m.spinner.Update(msg)
		m.ticks++
		next, cmd := m.flush()
		return next, tea.Batch(tick, cmd)
	}
	return m, nil
}

// drawn reports whether the first frame is on screen. The first tick comes
// from Init at once; the second comes a spinner interval later, after the
// first frame. A line printed before that frame overwrites earlier output.
func (m waitingModel) drawn() bool { return m.ticks > 1 }

func (m waitingModel) flush() (tea.Model, tea.Cmd) {
	if !m.drawn() || m.printing {
		return m, nil
	}
	if len(m.queue) > 0 {
		cmd := printLines(m.queue, waitingPrintedMsg{})
		m.queue, m.printing = nil, true
		return m, cmd
	}
	if m.stopping {
		m.stopped = true
		return m, tea.Quit
	}
	return m, nil
}

func (m waitingModel) View() tea.View {
	if m.stopped {
		return tea.NewView("")
	}
	line := m.spinner.View() + m.style.Render(" "+m.text)
	return tea.NewView(ansi.Truncate(line, m.width, "…"))
}
