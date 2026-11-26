package akvorado_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/akvorado"
)

func TestGetFlowSummary_Success(t *testing.T) {
	var gotPath, gotIface string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotIface = r.URL.Query().Get("interface")
		_, _ = w.Write([]byte(`{
			"interface": "sfp-sfpplus1",
			"in_bps": 4500000000,
			"out_bps": 1200000000,
			"top_talkers": [
				{"src_ip": "192.0.2.10", "bps": 950000000}
			]
		}`))
	}))
	defer srv.Close()

	client := akvorado.New(akvorado.Config{BaseURL: srv.URL + "/"})
	summary, err := client.GetFlowSummary(context.Background(), "sfp-sfpplus1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/api/v0/flows/summary" || gotIface != "sfp-sfpplus1" {
		t.Fatalf("unexpected request: path=%q iface=%q", gotPath, gotIface)
	}
	if summary.InBps != 4500000000 || len(summary.TopTalkers) != 1 || summary.TopTalkers[0].SrcIP != "192.0.2.10" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestQueryFlows_Success(t *testing.T) {
	var gotSrc, gotDst, gotDur string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		gotSrc = q.Get("src")
		gotDst = q.Get("dst")
		gotDur = q.Get("duration")
		_, _ = w.Write([]byte(`[
			{"src_ip": "192.0.2.10", "dst_ip": "198.51.100.20", "protocol": "UDP", "bytes": 1048576, "packets": 1024, "bps": 8388608}
		]`))
	}))
	defer srv.Close()

	client := akvorado.New(akvorado.Config{BaseURL: srv.URL})
	rows, err := client.QueryFlows(context.Background(), "192.0.2.10", "198.51.100.20", 10*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSrc != "192.0.2.10" || gotDst != "198.51.100.20" || gotDur != "10m0s" {
		t.Fatalf("unexpected query params: src=%q dst=%q dur=%q", gotSrc, gotDst, gotDur)
	}
	if len(rows) != 1 || rows[0].Protocol != "UDP" {
		t.Fatalf("unexpected flow rows: %+v", rows)
	}
}

func TestClient_ValidationAndHTTPError(t *testing.T) {
	unconf := akvorado.New(akvorado.Config{})
	if _, err := unconf.GetFlowSummary(context.Background(), "eth0"); err == nil {
		t.Fatal("expected error for unconfigured client")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer srv.Close()

	client := akvorado.New(akvorado.Config{BaseURL: srv.URL})
	if _, err := client.GetFlowSummary(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty interface")
	}
	if _, err := client.QueryFlows(context.Background(), "", "", 0); err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("expected HTTP 502 error, got %v", err)
	}
}
