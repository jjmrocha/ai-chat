package chat

import (
	"context"
	"sync"

	"github.com/jjmrocha/go-algo/queue"
)

type inbox struct {
	mu         sync.Mutex
	busy       bool
	idle       chan struct{}
	items      *queue.Queue[string]
	cancel     context.CancelCauseFunc
	cancelling bool
}

func newInbox() *inbox {
	idle := make(chan struct{})
	close(idle)
	return &inbox{items: queue.New[string](), idle: idle}
}

func (b *inbox) push(text string) (ctx context.Context, started bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.busy {
		b.items.Enqueue(text)
		return nil, false
	}
	b.busy = true
	b.idle = make(chan struct{})
	return b.arm(), true
}

func (b *inbox) next() (string, context.Context, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.disarm()
	text, ok := b.items.Dequeue()
	if !ok {
		b.busy = false
		close(b.idle)
		return "", nil, false
	}
	return text, b.arm(), true
}

func (b *inbox) arm() context.Context {
	ctx, cancel := context.WithCancelCause(context.Background())
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

func (b *inbox) drained() <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.idle
}

func (b *inbox) state() (busy, queued, cancelling bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.busy, !b.items.Empty(), b.cancelling
}
