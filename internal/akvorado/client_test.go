package akvorado_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/akvorado"
)

type capturedGraphReq struct {
	Start      string   `json:"start"`
	End        string   `json:"end"`
	Dimensions []string `json:"dimensions"`
	Filter     string   `json:"filter"`
	Limit      int      `json:"limit"`
	Points     int      `json:"points"`
	Units      string   `json:"units"`
}

func TestGetFlowSummary_Success(t *testing.T) {
	var gotPath, gotMethod string
	var gotReq capturedGraphReq
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotReq)
		_, _ = w.Write([]byte(`{
			"rows": [["192.0.2.10"], ["Other"]],
			"average": [950000000, 3550000000],
			"total": [28500000000, 106500000000]
		}`))
	}))
	defer srv.Close()

	client := akvorado.New(akvorado.Config{BaseURL: srv.URL + "/"})
	summary, err := client.GetFlowSummary(context.Background(), "sfp-sfpplus1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v0/console/graph/line" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if gotReq.Filter != `InIfName = "sfp-sfpplus1"` || gotReq.Units != "l3bps" || gotReq.Points != 10 {
		t.Fatalf("unexpected graph/line payload: %+v", gotReq)
	}
	if summary.InBps != 4500000000 || len(summary.TopTalkers) != 1 || summary.TopTalkers[0].SrcIP != "192.0.2.10" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestQueryFlows_Success(t *testing.T) {
	t.Run("ip filter", func(t *testing.T) {
		var gotReq capturedGraphReq
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotReq)
			_, _ = w.Write([]byte(`{
				"rows": [["192.0.2.10", "198.51.100.20"], ["Other", "Other"]],
				"average": [8388608, 1000000],
				"total": [8388608, 1000000]
			}`))
		}))
		defer srv.Close()

		client := akvorado.New(akvorado.Config{BaseURL: srv.URL})
		rows, err := client.QueryFlows(context.Background(), "192.0.2.10", "198.51.100.20", 10*time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.Filter != "SrcAddr = 192.0.2.10 AND DstAddr = 198.51.100.20" {
			t.Fatalf("unexpected IP filter: %q", gotReq.Filter)
		}
		if len(rows) != 1 || rows[0].SrcIP != "192.0.2.10" || rows[0].DstIP != "198.51.100.20" || rows[0].Bytes != 1048576 {
			t.Fatalf("unexpected flow rows: %+v", rows)
		}
	})

	t.Run("exporter hostname filter", func(t *testing.T) {
		var gotReq capturedGraphReq
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotReq)
			_, _ = w.Write([]byte(`{"rows":[["192.0.2.1","198.51.100.2"]],"average":[1000],"total":[8000]}`))
		}))
		defer srv.Close()

		client := akvorado.New(akvorado.Config{BaseURL: srv.URL})
		rows, err := client.QueryFlows(context.Background(), "CORE-MX204-01", "", 5*time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.Filter != `ExporterName = "CORE-MX204-01"` {
			t.Fatalf("unexpected exporter filter: %q", gotReq.Filter)
		}
		if len(rows) != 1 || rows[0].Bps != 1000 {
			t.Fatalf("unexpected rows: %+v", rows)
		}
	})
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
