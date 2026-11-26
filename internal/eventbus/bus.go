// Package eventbus provides a bounded, non-blocking in-memory event pipeline
// decoupling webhook ingestion from enrichment, rule evaluation, and dispatch.
package eventbus

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/akvorado"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/librenms"
)

const DefaultBufferSize = 256

var (
	// ErrBusFull is returned when Publish is called on a full channel buffer.
	ErrBusFull = errors.New("eventbus: buffer full")
	// ErrBusClosed is returned when Publish is called after Close.
	ErrBusClosed = errors.New("eventbus: bus closed")
)

// Event represents an inbound Zabbix event and its enriched context.
type Event struct {
	ID        string
	Name      string
	Host      string
	Severity  string
	Status    string // "PROBLEM" | "RESOLVED"
	Tags      map[string]string
	Timestamp time.Time

	Device *librenms.Device // nil if not found
	Flows  []akvorado.Flow  // empty if no traffic data
}

// Bus buffers events and drains them sequentially in a single worker goroutine.
type Bus struct {
	mu     sync.RWMutex
	closed bool
	ch     chan Event
}

// New creates an EventBus with the specified buffer size (defaults to 256 if <= 0).
func New(bufferSize int) *Bus {
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}
	return &Bus{
		ch: make(chan Event, bufferSize),
	}
}

// Publish enqueues an event without blocking. Never panics even if called concurrently with Close.
func (b *Bus) Publish(ev Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return ErrBusClosed
	}

	select {
	case b.ch <- ev:
		return nil
	default:
		return ErrBusFull
	}
}

// Close marks the bus closed and closes the underlying channel. Idempotent.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.closed {
		b.closed = true
		close(b.ch)
	}
}

// Run drains events in a single worker until ctx is cancelled or the bus is closed.
func (b *Bus) Run(ctx context.Context, handler func(context.Context, Event)) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-b.ch:
			if !ok {
				return
			}
			if handler != nil {
				handler(ctx, ev)
			}
		}
	}
}
