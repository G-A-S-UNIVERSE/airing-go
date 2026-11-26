package librenms

import (
	"context"
	"fmt"
	"sync"
)

// MockClient is a configurable LibreNMSClient for unit and integration tests.
//
// Behaviour is driven by the function fields: whichever is non-nil wins, and a
// nil function falls back to the static Devices slice. The call recorders are
// safe for concurrent use, so a single mock can be shared by several
// goroutines (e.g. a parallel enricher).
//
// Configure GetDeviceFn, ListDevicesFn and Devices before the mock is used
// concurrently; mutating them afterwards is a data race. The mock passes
// arguments through verbatim - it applies no defaults and no validation, so
// tests can assert on the exact input a caller produced.
type MockClient struct {
	// GetDeviceFn overrides the result of GetDevice when non-nil.
	GetDeviceFn func(ctx context.Context, hostname string) (*Device, error)

	// ListDevicesFn overrides the result of ListDevices when non-nil.
	ListDevicesFn func(ctx context.Context) ([]Device, error)

	// Devices is the fallback inventory used when the matching Fn is nil.
	// GetDevice returns the first entry whose Hostname matches exactly.
	Devices []Device

	mu               sync.Mutex
	getDeviceCalls   []string
	listDevicesCalls int
}

var _ LibreNMSClient = (*MockClient)(nil)

// GetDevice returns the device registered for hostname.
func (m *MockClient) GetDevice(ctx context.Context, hostname string) (*Device, error) {
	m.mu.Lock()
	m.getDeviceCalls = append(m.getDeviceCalls, hostname)
	fn := m.GetDeviceFn
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx, hostname)
	}

	for i := range m.Devices {
		if m.Devices[i].Hostname == hostname {
			dev := m.Devices[i]
			return &dev, nil
		}
	}
	return nil, fmt.Errorf("librenms mock: device %q not found", hostname)
}

// ListDevices returns the configured device inventory.
func (m *MockClient) ListDevices(ctx context.Context) ([]Device, error) {
	m.mu.Lock()
	m.listDevicesCalls++
	fn := m.ListDevicesFn
	m.mu.Unlock()

	if fn != nil {
		return fn(ctx)
	}

	out := make([]Device, len(m.Devices))
	copy(out, m.Devices)
	return out, nil
}

// GetDeviceCalls returns the hostnames passed to GetDevice, in call order.
func (m *MockClient) GetDeviceCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]string, len(m.getDeviceCalls))
	copy(out, m.getDeviceCalls)
	return out
}

// ListDevicesCalls returns how many times ListDevices was called.
func (m *MockClient) ListDevicesCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.listDevicesCalls
}

// Reset clears the recorded call history.
func (m *MockClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getDeviceCalls = nil
	m.listDevicesCalls = 0
}
