package notify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubNotifier struct {
	name   string
	called int
	err    error
}

func (s *stubNotifier) Name() string { return s.name }
func (s *stubNotifier) Notify(_ context.Context, _ Event) error {
	s.called++
	return s.err
}

func TestWebhookNotifier_SuccessAndBearer(t *testing.T) {
	var gotAuth, gotCT string
	var gotEvent Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &gotEvent); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	n := NewWebhook(srv.URL, "secret-token-xyz")
	if n.Name() != "webhook" {
		t.Fatalf("expected name=webhook, got %q", n.Name())
	}

	ev := Event{
		Kind:     "HIGH_BW",
		Severity: SeverityCritical,
		Source:   "zabbix",
		Message:  "HIGH_BW on 192.0.2.10",
		Metadata: map[string]string{"host": "192.0.2.10", "event_id": "101"},
	}
	if err := n.Notify(context.Background(), ev); err != nil {
		t.Fatalf("Notify error: %v", err)
	}
	if gotAuth != "Bearer secret-token-xyz" {
		t.Fatalf("expected Bearer auth header, got %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Fatalf("expected application/json, got %q", gotCT)
	}
	if gotEvent.Kind != ev.Kind || gotEvent.Severity != ev.Severity || gotEvent.Metadata["host"] != "192.0.2.10" {
		t.Fatalf("unexpected decoded event: %+v", gotEvent)
	}
}

func TestWebhookNotifier_Non2xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "downstream failure", http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := NewWebhook(srv.URL, "")
	err := n.Notify(context.Background(), Event{Kind: "TEST", Severity: SeverityInfo, Source: "zabbix", Message: "msg"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("expected HTTP 500 error, got %v", err)
	}
}

func TestMultiNotifier_CallsAllAndJoinsErrors(t *testing.T) {
	errBoom := errors.New("boom")
	s1 := &stubNotifier{name: "first", err: errBoom}
	s2 := &stubNotifier{name: "second"}

	m := Multi{s1, nil, s2}
	if m.Name() != "multi" {
		t.Fatalf("expected name=multi, got %q", m.Name())
	}

	err := m.Notify(context.Background(), Event{Kind: "TEST"})
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected joined error containing errBoom, got %v", err)
	}
	if s1.called != 1 || s2.called != 1 {
		t.Fatalf("expected both notifiers called once, got s1=%d s2=%d", s1.called, s2.called)
	}
}
