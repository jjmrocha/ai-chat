package chat

import (
	"context"
	"sync"

	"github.com/jjmrocha/go-algo/queue"
)

type inbox struct {
	mu         sync.Mutex
	busy       bool
	items      *queue.Queue[string]
	cancel     context.CancelCauseFunc
	cancelling bool
}

func newInbox() *inbox {
	return &inbox{items: queue.New[string]()}
}

func (b *inbox) push(parent context.Context, text string) (ctx context.Context, started bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.busy {
		b.items.Enqueue(text)
		return nil, false
	}
	b.busy = true
	return b.arm(parent), true
}

func (b *inbox) next(parent context.Context) (string, context.Context, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.disarm()
	text, ok := b.items.Dequeue()
	if !ok {
		b.busy = false
		return "", nil, false
	}
	return text, b.arm(parent), true
}

func (b *inbox) arm(parent context.Context) context.Context {
	ctx, cancel := context.WithCancelCause(parent)
	b.cancel = cancel
	b.cancelling = false
	return ctx
}

func (b *inbox) disarm() {
	if b.cancel != nil {
		b.cancel(nil)
	}
	b.cancel = nil
	b.cancelling = false
}

func (b *inbox) drop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.items = queue.New[string]()
	if b.cancel == nil || b.cancelling {
		return
	}
	b.cancelling = true
	b.cancel(errCancelled)
}

func (b *inbox) running() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.busy
}

func (b *inbox) waiting() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return !b.items.Empty()
}

func (b *inbox) cancelPending() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cancelling
}
