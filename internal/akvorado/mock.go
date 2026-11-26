package akvorado

import (
	"context"
	"sync"
	"time"
)

// QueryFlowsCall records the arguments of a single QueryFlows invocation.
type QueryFlowsCall struct {
	Src      string
	Dst      string
	Duration time.Duration
}

// MockClient is a configurable AkvoradoClient for unit and integration tests.
//
// Behaviour is driven by QueryFlowsFn: when it is non-nil its result is
// returned verbatim, otherwise the static Flows slice is returned. The call
// recorder is safe for concurrent use, so a single mock can be shared by
// several goroutines (e.g. a parallel enricher).
//
// Configure QueryFlowsFn and Flows before the mock is used concurrently;
// mutating them afterwards is a data race. Arguments are passed through
// verbatim - the mock applies no defaults, so a zero duration reaches the
// caller unchanged and tests can assert on exactly what was requested.
type MockClient struct {
	// QueryFlowsFn overrides the result of QueryFlows when non-nil.
	QueryFlowsFn func(ctx context.Context, src, dst string, duration time.Duration) ([]Flow, error)

	// Flows is the fallback result used when QueryFlowsFn is nil.
	Flows []Flow

	mu             sync.Mutex
	queryFlowsCall []QueryFlowsCall
}

var _ AkvoradoClient = (*MockClient)(nil)

// QueryFlows returns the flow records for the given filter.
func (m *MockClient) QueryFlows(ctx context.Context, src, dst string, duration time.Duration) ([]Flow, error) {
	m.mu.Lock()
	m.queryFlowsCall = append(m.queryFlowsCall, QueryFlowsCall{Src: src, Dst: dst, Duration: duration})
	fn := m.QueryFlowsFn
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, src, dst, duration)
	}

	out := make([]Flow, len(m.Flows))
	copy(out, m.Flows)
	return out, nil
}

// QueryFlowsCalls returns the recorded QueryFlows invocations, in call order.
func (m *MockClient) QueryFlowsCalls() []QueryFlowsCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]QueryFlowsCall, len(m.queryFlowsCall))
	copy(out, m.queryFlowsCall)
	return out
}

// Reset clears the recorded call history.
func (m *MockClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queryFlowsCall = nil
}
