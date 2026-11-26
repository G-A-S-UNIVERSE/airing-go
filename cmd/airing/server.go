package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/enricher"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/eventbus"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/olt"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/rules"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/webhook"
	"github.com/G-A-S-UNIVERSE/airing-go/pkg/notify"
)

const appVersion = "0.1.0"

type serverConfig struct {
	APIToken      string
	WebhookSecret string
	Bus           *eventbus.Bus
	Logger        *slog.Logger
}

type oltSummary struct {
	Key       string `json:"key"`
	Connected bool   `json:"connected"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func requireBearer(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if token == "" || !strings.HasPrefix(auth, prefix) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		got := strings.TrimPrefix(auth, prefix)
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func newMux(cfg serverConfig) http.Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"version": appVersion,
		})
	})

	mux.Handle("POST /webhooks/zabbix", webhook.NewZabbixHandler(cfg.WebhookSecret, cfg.Bus, logger))

	mux.HandleFunc("GET /olts", requireBearer(cfg.APIToken, func(w http.ResponseWriter, _ *http.Request) {
		keys := olt.Keys()
		sort.Strings(keys)
		out := make([]oltSummary, 0, len(keys))
		for _, k := range keys {
			out = append(out, oltSummary{Key: k, Connected: false})
		}
		writeJSON(w, http.StatusOK, out)
	}))

	mux.HandleFunc("POST /olts/{key}/reboot/{onuID}", requireBearer(cfg.APIToken, func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		onuID := r.PathValue("onuID")
		adapter, err := olt.New(key)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		if err := adapter.RebootONU(r.Context(), onuID); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}))

	return mux
}

type slogNotifier struct {
	logger *slog.Logger
}

func (n *slogNotifier) Name() string { return "slog" }

func (n *slogNotifier) Notify(_ context.Context, ev notify.Event) error {
	n.logger.Info("notify event dispatched",
		"kind", ev.Kind,
		"severity", ev.Severity,
		"source", ev.Source,
		"message", ev.Message,
	)
	return nil
}

func processEvent(
	ctx context.Context,
	ev eventbus.Event,
	enr *enricher.Enricher,
	eng *rules.Engine,
	notifier notify.Notifier,
	logger *slog.Logger,
) {
	if enr != nil {
		enr.Enrich(ctx, &ev)
	}
	act := eng.Evaluate(ev)

	if act.Type == "fetch_onus" && act.OLTKey != "" {
		adapter, err := olt.New(act.OLTKey)
		if err != nil {
			logger.Warn("worker: unknown OLT adapter", "olt_key", act.OLTKey, "error", err)
		} else if onus, err := adapter.ListONUs(ctx); err != nil {
			logger.Warn("worker: ListONUs failed", "olt_key", act.OLTKey, "error", err)
		} else {
			logger.Info("worker: fetched ONUs", "olt_key", act.OLTKey, "count", len(onus))
		}
	}

	if notifier != nil {
		sev := notify.SeverityInfo
		switch strings.ToLower(ev.Severity) {
		case "high", "disaster", "critical":
			sev = notify.SeverityCritical
		case "warning", "average":
			sev = notify.SeverityWarning
		}
		if err := notifier.Notify(ctx, notify.Event{
			Kind:     ev.Name,
			Severity: sev,
			Source:   "zabbix",
			Message:  act.Message,
			Metadata: ev.Tags,
		}); err != nil {
			logger.Error("worker: notify failed", "event_id", ev.ID, "error", err)
		}
	}
}
