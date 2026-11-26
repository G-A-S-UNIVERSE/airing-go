package librenms_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/librenms"
)

func TestGetDevices_Success(t *testing.T) {
	var gotPath, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-Auth-Token")
		_, _ = w.Write([]byte(`{
			"status": "ok",
			"devices": [
				{"device_id": 1, "hostname": "olt-region1-01", "sysName": "OLT-REGION1-01", "status": 1, "os": "zte", "hardware": "ZXA10 C320"},
				{"device_id": 2, "hostname": "core-sw-01", "sysName": "CORE-SW-01", "status": 1, "os": "routeros", "hardware": "CCR2116"}
			]
		}`))
	}))
	defer srv.Close()

	client := librenms.New(librenms.Config{BaseURL: srv.URL + "/", Token: "lnms-secret"})
	devices, err := client.GetDevices(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/api/v0/devices" {
		t.Fatalf("expected /api/v0/devices, got %q", gotPath)
	}
	if gotToken != "lnms-secret" {
		t.Fatalf("expected X-Auth-Token lnms-secret, got %q", gotToken)
	}
	if len(devices) != 2 || devices[0].Hostname != "olt-region1-01" {
		t.Fatalf("unexpected devices: %+v", devices)
	}

	listed, err := client.ListDevices(context.Background())
	if err != nil || len(listed) != 2 {
		t.Fatalf("expected ListDevices to match GetDevices, got %+v, err=%v", listed, err)
	}
}

func TestGetDevice_SuccessAndNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v0/devices/olt-region1-01":
			_, _ = w.Write([]byte(`{"status":"ok","devices":[{"device_id":1,"hostname":"olt-region1-01","status":1,"os":"zte"}]}`))
		case "/api/v0/devices/missing":
			_, _ = w.Write([]byte(`{"status":"ok","devices":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := librenms.New(librenms.Config{BaseURL: srv.URL, Token: "tok"})
	dev, err := client.GetDevice(context.Background(), "olt-region1-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dev.DeviceID != 1 || dev.OS != "zte" {
		t.Fatalf("unexpected device: %+v", dev)
	}

	if _, err := client.GetDevice(context.Background(), "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestClient_ValidationAndHTTPError(t *testing.T) {
	unconf := librenms.New(librenms.Config{})
	if _, err := unconf.GetDevices(context.Background()); err == nil {
		t.Fatal("expected error for unconfigured client")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := librenms.New(librenms.Config{BaseURL: srv.URL, Token: "bad"})
	if _, err := client.GetDevice(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty hostname")
	}
	err := func() error {
		_, e := client.GetDevices(context.Background())
		return e
	}()
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("expected HTTP 401 error, got %v", err)
	}
}
