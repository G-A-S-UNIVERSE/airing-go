package enricher_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/akvorado"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/enricher"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/eventbus"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/librenms"
)

type mockLibreNMS struct {
	fn func(ctx context.Context, hostname string) (*librenms.Device, error)
}

func (m *mockLibreNMS) GetDevice(ctx context.Context, hostname string) (*librenms.Device, error) {
	return m.fn(ctx, hostname)
}

func (m *mockLibreNMS) ListDevices(_ context.Context) ([]librenms.Device, error) {
	return nil, nil
}

type mockAkvorado struct {
	fn func(ctx context.Context, src, dst string, dur time.Duration) ([]akvorado.Flow, error)
}

func (m *mockAkvorado) QueryFlows(ctx context.Context, src, dst string, dur time.Duration) ([]akvorado.Flow, error) {
	return m.fn(ctx, src, dst, dur)
}

func TestEnrich_ParallelSuccess(t *testing.T) {
	var inFlight atomic.Int32
	var maxConcurrent atomic.Int32

	recordConcurrency := func() {
		cur := inFlight.Add(1)
		for {
			prev := maxConcurrent.Load()
			if cur <= prev || maxConcurrent.CompareAndSwap(prev, cur) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		inFlight.Add(-1)
	}

	lnms := &mockLibreNMS{fn: func(_ context.Context, host string) (*librenms.Device, error) {
		recordConcurrency()
		return &librenms.Device{DeviceID: 7, Hostname: host, OS: "zte"}, nil
	}}
	akv := &mockAkvorado{fn: func(_ context.Context, src, _ string, _ time.Duration) ([]akvorado.Flow, error) {
		recordConcurrency()
		return []akvorado.Flow{{SrcIP: src, Protocol: "TCP", Bps: 1000}}, nil
	}}

	e := enricher.New(lnms, akv, time.Second, nil)
	ev := eventbus.Event{ID: "1", Host: "olt-region1-01"}
	e.Enrich(context.Background(), &ev)

	if maxConcurrent.Load() < 2 {
		t.Fatalf("expected parallel execution (>=2 concurrent), got %d", maxConcurrent.Load())
	}
	if ev.Device == nil || ev.Device.Hostname != "olt-region1-01" {
		t.Fatalf("expected enriched device, got %+v", ev.Device)
	}
	if len(ev.Flows) != 1 || ev.Flows[0].Protocol != "TCP" {
		t.Fatalf("expected enriched flows, got %+v", ev.Flows)
	}
}

func TestEnrich_PartialFailureAndHardTimeout(t *testing.T) {
	lnms := &mockLibreNMS{fn: func(_ context.Context, _ string) (*librenms.Device, error) {
		return nil, errors.New("librenms down")
	}}
	akv := &mockAkvorado{fn: func(ctx context.Context, _, _ string, _ time.Duration) ([]akvorado.Flow, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}

	e := enricher.New(lnms, akv, 50*time.Millisecond, nil)
	start := time.Now()
	ev := eventbus.Event{ID: "2", Host: "olt-region1-01"}
	e.Enrich(context.Background(), &ev)

	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("expected hard timeout around 50ms, took %v", elapsed)
	}
	if ev.Device != nil || len(ev.Flows) != 0 {
		t.Fatalf("expected empty enrichment on errors, got device=%v flows=%v", ev.Device, ev.Flows)
	}
}
