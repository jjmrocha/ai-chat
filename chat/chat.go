package chat

import (
	"context"
	"strings"
	"sync"

	"github.com/jjmrocha/ai-chat/command"
	"github.com/jjmrocha/ai-toolkit/agent"
	"github.com/jjmrocha/ai-toolkit/llm"
	"github.com/jjmrocha/go-algo/treemap"
)

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
	name  string
	agent agentBackend

	commands     *treemap.Map[string, command.Command]
	telemetryFmt TelemetryFormatter
	statusFmt    StatusFormatter

	inbox   *inbox
	agentMu sync.Mutex
	work    sync.WaitGroup

	mu             sync.Mutex
	baseCtx        context.Context
	workCtx        context.Context
	closed         bool
	transcript     []Line
	epoch          uint64
	observer       Observer
	pendingTool    string
	pendingCommand string
	lastMeta       agent.Metadata
	model          modelState
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
		commands:     treemap.New[string, command.Command](strings.Compare),
		telemetryFmt: defaultTelemetryFormatter,
		statusFmt:    defaultStatusFormatter,
		inbox:        newInbox(),
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
func (c *Chat) SetContext(ctx context.Context) {
	c.mu.Lock()
	c.baseCtx = ctx
	c.mu.Unlock()
}

// Close ends the session: it cancels the running turn or command and any
// model lookup, discards queued input, silences the observer and waits for
// that work to return. The transcript stays readable; [Chat.Submit] does
// nothing afterwards. Close is safe to call more than once; ui.Run calls it for
// you when the user quits.
func (c *Chat) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.observer = nil
	stopLookup := c.model.cancel
	c.mu.Unlock()

	c.inbox.drop()
	if stopLookup != nil {
		stopLookup()
	}
	c.work.Wait()
}

func (c *Chat) context() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.baseCtx
}

func (c *Chat) register(cmd command.Command) {
	c.commands.Put(cmd.Name(), cmd)
}

// Name returns the name given to [New], for a front-end to display.
func (c *Chat) Name() string { return c.name }

// Commands returns every registered command, sorted by name. It implements
// [command.Registry] so /help can list them.
func (c *Chat) Commands() []command.Command {
	return c.commands.ToList()
}

// LastMetadata returns the metadata of the most recent successful turn. It is
// the zero value before the first turn completes and after [Chat.Clear].
func (c *Chat) LastMetadata() agent.Metadata {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastMeta
}
