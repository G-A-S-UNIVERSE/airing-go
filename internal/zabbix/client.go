// Package zabbix provides a client for the Zabbix JSON-RPC and REST API.
// Zabbix 8 is airing-go's primary event spine: triggers, problems,
// CEP correlations, and acknowledgements all flow through here.
package zabbix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const defaultTimeout = 15 * time.Second

// Client communicates with a Zabbix server over JSON-RPC 2.0.
type Client struct {
	url    string
	token  string
	http   *http.Client
	nextID atomic.Int64
}

// Config holds connection parameters.
type Config struct {
	URL        string // e.g. "http://zabbix.example.com"
	Token      string // API token (Zabbix >= 5.4)
	Timeout    time.Duration
	HTTPClient *http.Client
}

// New creates a Zabbix client.
func New(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = defaultTimeout
		}
		hc = &http.Client{Timeout: timeout}
	}
	return &Client{
		url:   strings.TrimRight(cfg.URL, "/"),
		token: cfg.Token,
		http:  hc,
	}
}

// Trigger represents a Zabbix trigger / problem.
type Trigger struct {
	TriggerID   string `json:"triggerid"`
	Description string `json:"description"`
	Priority    string `json:"priority"` // "0"=not classified ... "5"=disaster
	Value       string `json:"value"`    // "0"=OK "1"=PROBLEM
	HostName    string `json:"hostname"`
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
	ID      int64  `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
	ID      int64           `json:"id"`
}

func (c *Client) call(ctx context.Context, method string, params any, out any) error {
	if c.url == "" || c.token == "" {
		return fmt.Errorf("zabbix: client not configured")
	}

	reqBody := rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      c.nextID.Add(1),
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("zabbix: marshal %s request: %w", method, err)
	}

	endpoint := c.url + "/api_jsonrpc.php"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("zabbix: build %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json-rpc")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("zabbix: %s request failed: %w", method, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("zabbix: read %s response: %w", method, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("zabbix: %s HTTP %d: %s", method, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return fmt.Errorf("zabbix: decode %s response: %w", method, err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("zabbix: %s RPC error %d: %s (%s)", method, rpcResp.Error.Code, rpcResp.Error.Message, rpcResp.Error.Data)
	}
	if out != nil && len(rpcResp.Result) > 0 {
		if err := json.Unmarshal(rpcResp.Result, out); err != nil {
			return fmt.Errorf("zabbix: decode %s result: %w", method, err)
		}
	}
	return nil
}

type rawTrigger struct {
	TriggerID   string `json:"triggerid"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Value       string `json:"value"`
	Hosts       []struct {
		Host string `json:"host"`
		Name string `json:"name"`
	} `json:"hosts"`
}

// GetActiveTriggers returns all active (PROBLEM) triggers.
func (c *Client) GetActiveTriggers(ctx context.Context) ([]Trigger, error) {
	params := map[string]any{
		"output":        []string{"triggerid", "description", "priority", "value"},
		"filter":        map[string]int{"value": 1},
		"only_true":     true,
		"active":        true,
		"skipDependent": true,
		"selectHosts":   []string{"host", "name"},
		"sortfield":     "priority",
		"sortorder":     "DESC",
	}

	var raw []rawTrigger
	if err := c.call(ctx, "trigger.get", params, &raw); err != nil {
		return nil, err
	}

	triggers := make([]Trigger, 0, len(raw))
	for _, rt := range raw {
		hostName := ""
		if len(rt.Hosts) > 0 {
			hostName = rt.Hosts[0].Name
			if hostName == "" {
				hostName = rt.Hosts[0].Host
			}
		}
		triggers = append(triggers, Trigger{
			TriggerID:   rt.TriggerID,
			Description: rt.Description,
			Priority:    rt.Priority,
			Value:       rt.Value,
			HostName:    hostName,
		})
	}
	return triggers, nil
}

// AcknowledgeProblem acknowledges a problem by event ID.
// Zabbix action bitmask: 2 = acknowledge event, 4 = add message (6 = both).
func (c *Client) AcknowledgeProblem(ctx context.Context, eventID, message string) error {
	if strings.TrimSpace(eventID) == "" {
		return fmt.Errorf("zabbix: eventID is required")
	}

	action := 2
	params := map[string]any{
		"eventids": []string{eventID},
	}
	if message != "" {
		action = 6
		params["message"] = message
	}
	params["action"] = action

	return c.call(ctx, "event.acknowledge", params, nil)
}
