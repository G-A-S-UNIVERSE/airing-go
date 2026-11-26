// Package webhook handles authenticated inbound alert webhooks (Zabbix 7).
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/eventbus"
)

const (
	signatureHeader = "X-Zabbix-Signature"
	signaturePrefix = "sha256="
	maxBodyBytes    = 1 << 20 // 1 MiB
	maxLogBodyBytes = 1024
)

// ZabbixWebhookPayload is the inbound webhook JSON schema from Zabbix.
type ZabbixWebhookPayload struct {
	EventID   string            `json:"event_id"`
	EventName string            `json:"event_name"`
	Host      string            `json:"host"`
	Severity  string            `json:"severity"`
	Status    string            `json:"status"`
	Tags      map[string]string `json:"tags"`
	Timestamp int64             `json:"timestamp"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func verifyHMAC(secret string, body []byte, header string) bool {
	if secret == "" || !strings.HasPrefix(header, signaturePrefix) {
		return false
	}
	sigHex := strings.TrimPrefix(header, signaturePrefix)
	gotMAC, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(gotMAC, expectedMAC)
}

// NewZabbixHandler returns an http.Handler for POST /webhooks/zabbix.
func NewZabbixHandler(secret string, bus *eventbus.Bus, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
			return
		}

		if !verifyHMAC(secret, body, r.Header.Get(signatureHeader)) {
			logger.Warn("zabbix webhook rejected: invalid signature", "remote_addr", r.RemoteAddr)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
			return
		}

		var payload ZabbixWebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			snippet := string(body)
			if len(snippet) > maxLogBodyBytes {
				snippet = snippet[:maxLogBodyBytes]
			}
			logger.Warn("zabbix webhook rejected: invalid payload", "error", err, "body_prefix_bytes", len(snippet))
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}

		ts := time.Unix(payload.Timestamp, 0).UTC()
		if payload.Timestamp <= 0 {
			ts = time.Now().UTC()
		}

		ev := eventbus.Event{
			ID:        payload.EventID,
			Name:      payload.EventName,
			Host:      payload.Host,
			Severity:  payload.Severity,
			Status:    payload.Status,
			Tags:      payload.Tags,
			Timestamp: ts,
		}

		if err := bus.Publish(ev); err != nil {
			statusCode := http.StatusServiceUnavailable
			if errors.Is(err, eventbus.ErrBusClosed) {
				statusCode = http.StatusServiceUnavailable
			}
			logger.Error("zabbix webhook publish failed", "event_id", payload.EventID, "error", err)
			writeJSON(w, statusCode, map[string]string{"error": "event queue unavailable"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
}
