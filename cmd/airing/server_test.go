package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/eventbus"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/olt"
	_ "github.com/G-A-S-UNIVERSE/airing-go/internal/olt/generic"
	_ "github.com/G-A-S-UNIVERSE/airing-go/internal/olt/zte"
)

type mockOLT struct{}

func (m *mockOLT) VendorKey() string                                          { return "mock_olt" }
func (m *mockOLT) Connect(_ context.Context) error                            { return nil }
func (m *mockOLT) Disconnect(_ context.Context) error                         { return nil }
func (m *mockOLT) ListONUs(_ context.Context) ([]olt.ONUStatus, error)        { return nil, nil }
func (m *mockOLT) GetONU(_ context.Context, _ string) (*olt.ONUStatus, error) { return nil, nil }
func (m *mockOLT) RebootONU(_ context.Context, _ string) error                { return nil }

func TestServer_HealthAndOLTs(t *testing.T) {
	olt.Register("mock_olt", func() olt.BaseOLTAdapter { return &mockOLT{} })
	bus := eventbus.New(16)
	h := newMux(serverConfig{
		APIToken:      "api-secret",
		WebhookSecret: "wh-secret",
		Bus:           bus,
	})

	// 1. GET /health (no auth)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected /health response: code=%d body=%s", rec.Code, rec.Body.String())
	}

	// 2. GET /olts without Bearer token -> 401
	req = httptest.NewRequest(http.MethodGet, "/olts", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	// 3. GET /olts with Bearer token -> 200 JSON array
	req = httptest.NewRequest(http.MethodGet, "/olts", nil)
	req.Header.Set("Authorization", "Bearer api-secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var olts []oltSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &olts); err != nil || len(olts) < 2 {
		t.Fatalf("expected >=2 OLT adapters, got %v (err=%v)", olts, err)
	}

	// 4. POST /olts/unknown/reboot/1 -> 404
	req = httptest.NewRequest(http.MethodPost, "/olts/unknown/reboot/1", nil)
	req.Header.Set("Authorization", "Bearer api-secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown OLT, got %d", rec.Code)
	}

	// 5a. POST /olts/mock_olt/reboot/1 -> 200
	req = httptest.NewRequest(http.MethodPost, "/olts/mock_olt/reboot/1", nil)
	req.Header.Set("Authorization", "Bearer api-secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("expected 200 for mock_olt reboot, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5b. POST /olts/zte_c320/reboot/1 -> 502 (stub unimplemented)
	req = httptest.NewRequest(http.MethodPost, "/olts/zte_c320/reboot/1", nil)
	req.Header.Set("Authorization", "Bearer api-secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for unimplemented zte_c320 reboot, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. POST /webhooks/zabbix with valid HMAC -> 200
	body := `{"event_id":"e1","event_name":"OLT_DOWN","host":"olt-01","status":"PROBLEM"}`
	mac := hmac.New(sha256.New, []byte("wh-secret"))
	mac.Write([]byte(body))
	req = httptest.NewRequest(http.MethodPost, "/webhooks/zabbix", strings.NewReader(body))
	req.Header.Set("X-Zabbix-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for webhook, got %d: %s", rec.Code, rec.Body.String())
	}
}
