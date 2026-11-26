package akvorado_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/akvorado"
)

// TestMockClientSatisfiesInterface proves the mock is usable from outside
// the package, which is how the enricher consumes it.
func TestMockClientSatisfiesInterface(t *testing.T) {
	var c akvorado.AkvoradoClient = &akvorado.MockClient{}
	if _, err := c.QueryFlows(context.Background(), "", "", 0); err != nil {
		t.Fatalf("unexpected error from default MockClient: %v", err)
	}
}

func TestMockClientQueryFlowsUsesConfiguredFn(t *testing.T) {
	want := []akvorado.Flow{{SrcIP: "10.0.0.1", DstIP: "10.0.0.2", Protocol: "tcp", Bytes: 1024}}

	var gotSrc, gotDst string
	var gotDuration time.Duration
	m := &akvorado.MockClient{
		QueryFlowsFn: func(_ context.Context, src, dst string, duration time.Duration) ([]akvorado.Flow, error) {
			gotSrc, gotDst, gotDuration = src, dst, duration
			return want, nil
		},
	}

	got, err := m.QueryFlows(context.Background(), "10.0.0.1", "10.0.0.2", 5*time.Minute)
	if err != nil {
		t.Fatalf("QueryFlows error: %v", err)
	}
	if len(got) != 1 || got[0].Bytes != 1024 {
		t.Errorf("flows = %+v, want one record with 1024 bytes", got)
	}
	if gotSrc != "10.0.0.1" || gotDst != "10.0.0.2" {
		t.Errorf("src/dst = %q/%q, want 10.0.0.1/10.0.0.2", gotSrc, gotDst)
	}
	if gotDuration != 5*time.Minute {
		t.Errorf("duration = %v, want 5m", gotDuration)
	}
}

func TestMockClientQueryFlowsFallsBackToFlows(t *testing.T) {
	m := &akvorado.MockClient{
		Flows: []akvorado.Flow{
			{SrcIP: "10.0.0.1", DstIP: "10.0.0.2", Protocol: "udp", Packets: 10},
			{SrcIP: "10.0.0.3", DstIP: "10.0.0.4", Protocol: "tcp", Packets: 20},
		},
	}

	got, err := m.QueryFlows(context.Background(), "", "", 0)
	if err != nil {
		t.Fatalf("QueryFlows error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(flows) = %d, want 2", len(got))
	}
}

func TestMockClientQueryFlowsPropagatesError(t *testing.T) {
	sentinel := errors.New("akvorado unavailable")
	m := &akvorado.MockClient{
		QueryFlowsFn: func(context.Context, string, string, time.Duration) ([]akvorado.Flow, error) {
			return nil, sentinel
		},
	}

	if _, err := m.QueryFlows(context.Background(), "a", "b", time.Minute); !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
}

func TestMockClientRecordsQueryFlowsCalls(t *testing.T) {
	m := &akvorado.MockClient{}

	if _, err := m.QueryFlows(context.Background(), "10.0.0.1", "10.0.0.2", 5*time.Minute); err != nil {
		t.Fatalf("QueryFlows error: %v", err)
	}
	if _, err := m.QueryFlows(context.Background(), "10.0.0.3", "", time.Hour); err != nil {
		t.Fatalf("QueryFlows error: %v", err)
	}

	calls := m.QueryFlowsCalls()
	if len(calls) != 2 {
		t.Fatalf("len(calls) = %d, want 2", len(calls))
	}
	if calls[0].Src != "10.0.0.1" || calls[0].Dst != "10.0.0.2" || calls[0].Duration != 5*time.Minute {
		t.Errorf("calls[0] = %+v, want src 10.0.0.1 dst 10.0.0.2 duration 5m", calls[0])
	}
	if calls[1].Src != "10.0.0.3" || calls[1].Dst != "" || calls[1].Duration != time.Hour {
		t.Errorf("calls[1] = %+v, want src 10.0.0.3 dst empty duration 1h", calls[1])
	}
}

func TestMockClientResetClearsCalls(t *testing.T) {
	m := &akvorado.MockClient{}
	if _, err := m.QueryFlows(context.Background(), "a", "b", time.Minute); err != nil {
		t.Fatalf("QueryFlows error: %v", err)
	}

	m.Reset()
	if got := len(m.QueryFlowsCalls()); got != 0 {
		t.Errorf("len(QueryFlowsCalls()) = %d, want 0", got)
	}
}

func TestMockClientConcurrentUse(t *testing.T) {
	m := &akvorado.MockClient{Flows: []akvorado.Flow{{SrcIP: "10.0.0.1"}}}

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if _, err := m.QueryFlows(context.Background(), "10.0.0.1", "10.0.0.2", time.Minute); err != nil {
				t.Errorf("QueryFlows error: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := len(m.QueryFlowsCalls()); got != goroutines {
		t.Errorf("len(QueryFlowsCalls()) = %d, want %d", got, goroutines)
	}
}
