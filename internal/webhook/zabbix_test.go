package webhook_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/eventbus"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/webhook"
)

func signBody(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestZabbixWebhook_ValidHMAC(t *testing.T) {
	const secret = "super-secret-key"
	bus := eventbus.New(16)
	h := webhook.NewZabbixHandler(secret, bus, nil)

	body := `{"event_id":"123","event_name":"OLT_DOWN","host":"olt-01","severity":"high","status":"PROBLEM","tags":{"olt_key":"zte_c320"},"timestamp":1700000000}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/zabbix", strings.NewReader(body))
	req.Header.Set("X-Zabbix-Signature", signBody(secret, body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("expected ok response, got %s", rec.Body.String())
	}
}

func TestZabbixWebhook_InvalidSignature(t *testing.T) {
	bus := eventbus.New(16)
	h := webhook.NewZabbixHandler("super-secret-key", bus, nil)

	body := `{"event_id":"123"}`
	for _, sig := range []string{"", "sha256=deadbeef", "invalid-prefix"} {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/zabbix", strings.NewReader(body))
		if sig != "" {
			req.Header.Set("X-Zabbix-Signature", sig)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("sig=%q: expected 401, got %d", sig, rec.Code)
		}
	}
}

func TestZabbixWebhook_MalformedJSON(t *testing.T) {
	const secret = "super-secret-key"
	bus := eventbus.New(16)
	h := webhook.NewZabbixHandler(secret, bus, nil)

	body := `{not-valid-json`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/zabbix", strings.NewReader(body))
	req.Header.Set("X-Zabbix-Signature", signBody(secret, body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
