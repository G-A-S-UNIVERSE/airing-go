package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/akvorado"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/enricher"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/eventbus"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/librenms"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/olt"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/rules"
	"github.com/G-A-S-UNIVERSE/airing-go/pkg/notify"
)

type integrationOLT struct {
	listCalls *atomic.Int32
}

func (o *integrationOLT) VendorKey() string                  { return "integration_olt" }
func (o *integrationOLT) Connect(_ context.Context) error    { return nil }
func (o *integrationOLT) Disconnect(_ context.Context) error { return nil }
func (o *integrationOLT) GetONU(_ context.Context, _ string) (*olt.ONUStatus, error) {
	return nil, nil
}
func (o *integrationOLT) RebootONU(_ context.Context, _ string) error { return nil }
func (o *integrationOLT) ListONUs(_ context.Context) ([]olt.ONUStatus, error) {
	o.listCalls.Add(1)
	return []olt.ONUStatus{{ONUID: "1/1/1:1", Status: "offline"}}, nil
}

type captureNotifier struct {
	ch chan notify.Event
}

func (c *captureNotifier) Name() string { return "capture" }
func (c *captureNotifier) Notify(_ context.Context, ev notify.Event) error {
	c.ch <- ev
	return nil
}

func TestEndToEnd_ZabbixWebhookPipeline(t *testing.T) {
	var oltListCalls atomic.Int32
	olt.Register("integration_olt", func() olt.BaseOLTAdapter {
		return &integrationOLT{listCalls: &oltListCalls}
	})

	lnmsMock := &librenms.MockClient{
		Devices: []librenms.Device{
			{DeviceID: 42, Hostname: "olt-region1-01", Status: 1, OS: "zte"},
		},
	}
	akvMock := &akvorado.MockClient{
		Flows: []akvorado.Flow{
			{SrcIP: "192.0.2.10", DstIP: "198.51.100.20", Protocol: "TCP", Bps: 1000},
		},
	}

	bus := eventbus.New(16)
	enr := enricher.New(lnmsMock, akvMock, 2*time.Second, slog.Default())
	eng := rules.New()
	notifier := &captureNotifier{ch: make(chan notify.Event, 4)}

	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		bus.Run(ctx, func(wCtx context.Context, ev eventbus.Event) {
			processEvent(wCtx, ev, enr, eng, notifier, slog.Default())
		})
	}()
	defer func() {
		cancel()
		bus.Close()
		<-workerDone
	}()

	const whSecret = "integration-wh-secret"
	handler := newMux(serverConfig{
		APIToken:      "integration-api-token",
		WebhookSecret: whSecret,
		Bus:           bus,
		Logger:        slog.Default(),
	})

	payload := `{"event_id":"ev-900","event_name":"OLT_DOWN","host":"olt-region1-01","severity":"high","status":"PROBLEM","tags":{"olt_key":"integration_olt"},"timestamp":1700000000}`
	mac := hmac.New(sha256.New, []byte(whSecret))
	mac.Write([]byte(payload))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/zabbix", strings.NewReader(payload))
	req.Header.Set("X-Zabbix-Signature", sig)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	select {
	case ev := <-notifier.ch:
		if ev.Kind != "OLT_DOWN" || ev.Severity != notify.SeverityCritical {
			t.Fatalf("unexpected notified event: %+v", ev)
		}
		if !strings.Contains(ev.Message, "integration_olt") {
			t.Fatalf("expected message to mention integration_olt, got %q", ev.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for notified event")
	}

	if calls := lnmsMock.GetDeviceCalls(); len(calls) != 1 || calls[0] != "olt-region1-01" {
		t.Fatalf("unexpected LibreNMS GetDeviceCalls: %v", calls)
	}
	if calls := akvMock.QueryFlowsCalls(); len(calls) != 1 || calls[0].Src != "olt-region1-01" {
		t.Fatalf("unexpected Akvorado QueryFlowsCalls: %+v", calls)
	}
	if got := oltListCalls.Load(); got != 1 {
		t.Fatalf("expected ListONUs to be called once, got %d", got)
	}
}
