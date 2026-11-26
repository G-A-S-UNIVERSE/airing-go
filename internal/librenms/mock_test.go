package librenms_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/librenms"
)

// TestMockClientSatisfiesInterface proves the mock is usable from outside
// the package, which is how the enricher consumes it.
func TestMockClientSatisfiesInterface(t *testing.T) {
	var c librenms.LibreNMSClient = &librenms.MockClient{}
	if _, err := c.ListDevices(context.Background()); err != nil {
		t.Fatalf("unexpected error from default MockClient: %v", err)
	}
}

func TestMockClientGetDeviceUsesConfiguredFn(t *testing.T) {
	want := &librenms.Device{DeviceID: 7, Hostname: "olt-region1-01", Status: 1}
	m := &librenms.MockClient{
		GetDeviceFn: func(_ context.Context, hostname string) (*librenms.Device, error) {
			if hostname != "olt-region1-01" {
				t.Errorf("hostname = %q, want %q", hostname, "olt-region1-01")
			}
			return want, nil
		},
	}

	got, err := m.GetDevice(context.Background(), "olt-region1-01")
	if err != nil {
		t.Fatalf("GetDevice error: %v", err)
	}
	if got != want {
		t.Errorf("GetDevice = %+v, want %+v", got, want)
	}

	calls := m.GetDeviceCalls()
	if len(calls) != 1 || calls[0] != "olt-region1-01" {
		t.Errorf("GetDeviceCalls() = %v, want [olt-region1-01]", calls)
	}
}

func TestMockClientGetDeviceFallsBackToDevices(t *testing.T) {
	m := &librenms.MockClient{
		Devices: []librenms.Device{
			{DeviceID: 1, Hostname: "olt-a", Status: 1},
			{DeviceID: 2, Hostname: "olt-b", Status: 0},
		},
	}

	got, err := m.GetDevice(context.Background(), "olt-b")
	if err != nil {
		t.Fatalf("GetDevice error: %v", err)
	}
	if got.DeviceID != 2 {
		t.Errorf("DeviceID = %d, want 2", got.DeviceID)
	}
}

func TestMockClientGetDeviceNotFound(t *testing.T) {
	m := &librenms.MockClient{Devices: []librenms.Device{{Hostname: "olt-a"}}}

	if _, err := m.GetDevice(context.Background(), "missing"); err == nil {
		t.Fatal("expected error for unknown device, got nil")
	}
}

func TestMockClientListDevicesFallsBackToDevices(t *testing.T) {
	m := &librenms.MockClient{
		Devices: []librenms.Device{{Hostname: "olt-a"}, {Hostname: "olt-b"}},
	}

	devices, err := m.ListDevices(context.Background())
	if err != nil {
		t.Fatalf("ListDevices error: %v", err)
	}
	if len(devices) != 2 {
		t.Errorf("len(devices) = %d, want 2", len(devices))
	}
	if got := m.ListDevicesCalls(); got != 1 {
		t.Errorf("ListDevicesCalls() = %d, want 1", got)
	}
}

func TestMockClientListDevicesUsesConfiguredFn(t *testing.T) {
	m := &librenms.MockClient{
		ListDevicesFn: func(context.Context) ([]librenms.Device, error) {
			return []librenms.Device{{Hostname: "from-fn"}}, nil
		},
	}

	devices, err := m.ListDevices(context.Background())
	if err != nil {
		t.Fatalf("ListDevices error: %v", err)
	}
	if len(devices) != 1 || devices[0].Hostname != "from-fn" {
		t.Errorf("devices = %+v, want one device named from-fn", devices)
	}
}

func TestMockClientPropagatesError(t *testing.T) {
	sentinel := errors.New("librenms unavailable")
	m := &librenms.MockClient{
		GetDeviceFn: func(context.Context, string) (*librenms.Device, error) {
			return nil, sentinel
		},
	}

	if _, err := m.GetDevice(context.Background(), "olt-a"); !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
}

func TestMockClientResetClearsCounters(t *testing.T) {
	m := &librenms.MockClient{}
	if _, err := m.GetDevice(context.Background(), "olt-a"); err == nil {
		t.Fatal("expected error for unknown device, got nil")
	}
	if _, err := m.ListDevices(context.Background()); err != nil {
		t.Fatalf("ListDevices error: %v", err)
	}

	m.Reset()
	if got := len(m.GetDeviceCalls()); got != 0 {
		t.Errorf("len(GetDeviceCalls()) = %d, want 0", got)
	}
	if got := m.ListDevicesCalls(); got != 0 {
		t.Errorf("ListDevicesCalls() = %d, want 0", got)
	}
}

func TestMockClientConcurrentUse(t *testing.T) {
	m := &librenms.MockClient{Devices: []librenms.Device{{Hostname: "olt-a"}}}

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if _, err := m.GetDevice(context.Background(), "olt-a"); err != nil {
				t.Errorf("GetDevice error: %v", err)
			}
			if _, err := m.ListDevices(context.Background()); err != nil {
				t.Errorf("ListDevices error: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := len(m.GetDeviceCalls()); got != goroutines {
		t.Errorf("len(GetDeviceCalls()) = %d, want %d", got, goroutines)
	}
	if got := m.ListDevicesCalls(); got != goroutines {
		t.Errorf("ListDevicesCalls() = %d, want %d", got, goroutines)
	}
}
