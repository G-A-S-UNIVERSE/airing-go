package eventbus_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/eventbus"
)

func TestBus_PublishAndDrain100Events(t *testing.T) {
	bus := eventbus.New(256)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var processed atomic.Int64
	done := make(chan struct{})
	go func() {
		bus.Run(ctx, func(_ context.Context, _ eventbus.Event) {
			if processed.Add(1) == 100 {
				close(done)
			}
		})
	}()

	for i := range 100 {
		if err := bus.Publish(eventbus.Event{
			ID:     fmt.Sprintf("ev-%d", i),
			Status: "PROBLEM",
		}); err != nil {
			t.Fatalf("publish %d failed: %v", i, err)
		}
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for 100 events; processed=%d", processed.Load())
	}
}

func TestBus_FullChannelReturnsError(t *testing.T) {
	bus := eventbus.New(2)
	if err := bus.Publish(eventbus.Event{ID: "1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := bus.Publish(eventbus.Event{ID: "2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := bus.Publish(eventbus.Event{ID: "3"}); !errors.Is(err, eventbus.ErrBusFull) {
		t.Fatalf("expected ErrBusFull, got %v", err)
	}
}

func TestBus_PublishAfterCloseReturnsError(t *testing.T) {
	bus := eventbus.New(4)
	bus.Close()
	bus.Close() // idempotent

	if err := bus.Publish(eventbus.Event{ID: "after-close"}); !errors.Is(err, eventbus.ErrBusClosed) {
		t.Fatalf("expected ErrBusClosed, got %v", err)
	}
}
