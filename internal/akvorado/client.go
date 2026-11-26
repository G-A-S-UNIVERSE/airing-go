// Package akvorado provides a client for the Akvorado flow analytics API.
// Akvorado is airing-go's traffic intelligence layer: NetFlow/sFlow anomaly
// detection, top-talker analysis, and upstream saturation alerts.
package akvorado

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout       = 30 * time.Second
	defaultQueryDuration = 15 * time.Minute
)

// Client communicates with an Akvorado instance.
type Client struct {
	baseURL string
	http    *http.Client
}

// Config holds connection parameters.
type Config struct {
	BaseURL    string // e.g. "http://akvorado.example.com"
	Timeout    time.Duration
	HTTPClient *http.Client
}

// New creates an Akvorado client.
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
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		http:    hc,
	}
}

// FlowSummary holds aggregated flow statistics for a time window.
type FlowSummary struct {
	Interface  string  `json:"interface"`
	InBps      float64 `json:"in_bps"`
	OutBps     float64 `json:"out_bps"`
	TopTalkers []struct {
		SrcIP string  `json:"src_ip"`
		Bps   float64 `json:"bps"`
	} `json:"top_talkers,omitempty"`
}

// FlowRecord represents a single flow query result row.
type FlowRecord struct {
	SrcIP    string  `json:"src_ip"`
	DstIP    string  `json:"dst_ip"`
	Protocol string  `json:"protocol"`
	Bytes    uint64  `json:"bytes"`
	Packets  uint64  `json:"packets"`
	Bps      float64 `json:"bps"`
}

func (c *Client) doGet(ctx context.Context, pathWithQuery string, out any) error {
	if c.baseURL == "" || c.http == nil {
		return fmt.Errorf("akvorado: client not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+pathWithQuery, nil)
	if err != nil {
		return fmt.Errorf("akvorado: build request %s: %w", pathWithQuery, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("akvorado: GET %s failed: %w", pathWithQuery, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("akvorado: read %s response: %w", pathWithQuery, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("akvorado: GET %s HTTP %d: %s", pathWithQuery, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("akvorado: decode %s response: %w", pathWithQuery, err)
	}
	return nil
}

// GetFlowSummary returns flow stats for the given interface over the last window.
func (c *Client) GetFlowSummary(ctx context.Context, iface string) (*FlowSummary, error) {
	trimmed := strings.TrimSpace(iface)
	if trimmed == "" {
		return nil, fmt.Errorf("akvorado: interface is required")
	}

	q := url.Values{}
	q.Set("interface", trimmed)

	var summary FlowSummary
	if err := c.doGet(ctx, "/api/v0/flows/summary?"+q.Encode(), &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

// QueryFlows queries flow records filtered by optional source, destination, and time window.
func (c *Client) QueryFlows(ctx context.Context, src, dst string, duration time.Duration) ([]FlowRecord, error) {
	if duration <= 0 {
		duration = defaultQueryDuration
	}

	q := url.Values{}
	if s := strings.TrimSpace(src); s != "" {
		q.Set("src", s)
	}
	if d := strings.TrimSpace(dst); d != "" {
		q.Set("dst", d)
	}
	q.Set("duration", duration.String())

	var records []FlowRecord
	if err := c.doGet(ctx, "/api/v0/flows?"+q.Encode(), &records); err != nil {
		return nil, err
	}
	return records, nil
}
