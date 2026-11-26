package zabbix_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/zabbix"
)

type capturedRPC struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
	ID      int64          `json:"id"`
}

func TestGetActiveTriggers_Success(t *testing.T) {
	var got capturedRPC
	var gotAuth, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)

		w.Header().Set("Content-Type", "application/json-rpc")
		_, _ = w.Write([]byte(`{
			"jsonrpc": "2.0",
			"result": [
				{
					"triggerid": "101",
					"description": "PON port optical power low",
					"priority": "4",
					"value": "1",
					"hosts": [{"host": "olt-region1-01", "name": "OLT-REGION1-01"}]
				},
				{
					"triggerid": "102",
					"description": "High CPU utilization",
					"priority": "2",
					"value": "1",
					"hosts": [{"host": "core-sw-01", "name": ""}]
				}
			],
			"id": 1
		}`))
	}))
	defer srv.Close()

	client := zabbix.New(zabbix.Config{URL: srv.URL + "/", Token: "test-token"})
	triggers, err := client.GetActiveTriggers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/api_jsonrpc.php" {
		t.Fatalf("expected path /api_jsonrpc.php, got %q", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("expected Bearer auth header, got %q", gotAuth)
	}
	if got.Method != "trigger.get" {
		t.Fatalf("expected method trigger.get, got %q", got.Method)
	}
	if len(triggers) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(triggers))
	}
	if triggers[0].TriggerID != "101" || triggers[0].HostName != "OLT-REGION1-01" {
		t.Fatalf("unexpected first trigger: %+v", triggers[0])
	}
	if triggers[1].HostName != "core-sw-01" {
		t.Fatalf("expected fallback to host field, got %q", triggers[1].HostName)
	}
}

func TestGetActiveTriggers_RPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"jsonrpc": "2.0",
			"error": {"code": -32602, "message": "Invalid params.", "data": "Not authorized."},
			"id": 1
		}`))
	}))
	defer srv.Close()

	client := zabbix.New(zabbix.Config{URL: srv.URL, Token: "bad-token"})
	_, err := client.GetActiveTriggers(context.Background())
	if err == nil || !strings.Contains(err.Error(), "RPC error -32602") {
		t.Fatalf("expected RPC error -32602, got %v", err)
	}
}

func TestGetActiveTriggers_NotConfigured(t *testing.T) {
	client := zabbix.New(zabbix.Config{})
	if _, err := client.GetActiveTriggers(context.Background()); err == nil {
		t.Fatal("expected error for unconfigured client")
	}
}

func TestAcknowledgeProblem_Success(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		wantAction float64
	}{
		{name: "with message", message: "Investigating fiber cut", wantAction: 6},
		{name: "without message", message: "", wantAction: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got capturedRPC
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &got)
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"eventids":["9001"]},"id":1}`))
			}))
			defer srv.Close()

			client := zabbix.New(zabbix.Config{URL: srv.URL, Token: "test-token"})
			if err := client.AcknowledgeProblem(context.Background(), "9001", tc.message); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Method != "event.acknowledge" {
				t.Fatalf("expected method event.acknowledge, got %q", got.Method)
			}
			if got.Params["action"] != tc.wantAction {
				t.Fatalf("expected action %v, got %v", tc.wantAction, got.Params["action"])
			}
		})
	}
}

func TestAcknowledgeProblem_ValidationAndHTTPError(t *testing.T) {
	client := zabbix.New(zabbix.Config{URL: "http://127.0.0.1:1", Token: "t"})
	if err := client.AcknowledgeProblem(context.Background(), "  ", "msg"); err == nil {
		t.Fatal("expected error for empty eventID")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client = zabbix.New(zabbix.Config{URL: srv.URL, Token: "t"})
	err := client.AcknowledgeProblem(context.Background(), "9001", "ack")
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("expected HTTP 503 error, got %v", err)
	}
}
