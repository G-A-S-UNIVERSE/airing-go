// Package akvorado provides a client for the Akvorado flow analytics API.
// Akvorado is airing-go's traffic intelligence layer: NetFlow/sFlow anomaly
// detection, top-talker analysis, and upstream saturation alerts.
package akvorado

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTimeout       = 30 * time.Second
	defaultQueryDuration = 15 * time.Minute
	defaultFlowLimit     = 10
	defaultGraphPoints   = 10
)

// Flow is the canonical flow record type exported for event enrichment.
type Flow = FlowRecord

// AkvoradoClient defines the flow-query contract used by enrichment workers and mocks.
type AkvoradoClient interface {
	QueryFlows(ctx context.Context, src, dst string, duration time.Duration) ([]Flow, error)
}

var _ AkvoradoClient = (*Client)(nil)

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

// TopTalker holds a source address and its average bit rate.
type TopTalker struct {
	SrcIP string  `json:"src_ip"`
	Bps   float64 `json:"bps"`
}

// FlowSummary holds aggregated flow statistics for a time window.
type FlowSummary struct {
	Interface  string      `json:"interface"`
	InBps      float64     `json:"in_bps"`
	OutBps     float64     `json:"out_bps"`
	TopTalkers []TopTalker `json:"top_talkers,omitempty"`
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

type graphLineRequest struct {
	Start      string   `json:"start"`
	End        string   `json:"end"`
	Dimensions []string `json:"dimensions"`
	Filter     string   `json:"filter,omitempty"`
	Limit      int      `json:"limit"`
	Points     int      `json:"points"`
	Units      string   `json:"units"`
}

type graphLineResponse struct {
	Rows    [][]string `json:"rows"`
	Average []float64  `json:"average"`
	Total   []float64  `json:"total"`
	Last    []float64  `json:"last"`
}

func sanitizeFilterValue(s string) string {
	s = strings.ReplaceAll(s, `"`, "")
	s = strings.ReplaceAll(s, `\`, "")
	return strings.TrimSpace(s)
}

func buildFilter(src, dst string) string {
	var clauses []string
	if s := sanitizeFilterValue(src); s != "" {
		if net.ParseIP(s) != nil {
			clauses = append(clauses, "SrcAddr = "+s)
		} else {
			clauses = append(clauses, fmt.Sprintf(`ExporterName = "%s"`, s))
		}
	}
	if d := sanitizeFilterValue(dst); d != "" {
		if net.ParseIP(d) != nil {
			clauses = append(clauses, "DstAddr = "+d)
		} else {
			clauses = append(clauses, fmt.Sprintf(`DstASName = "%s"`, d))
		}
	}
	return strings.Join(clauses, " AND ")
}

func (c *Client) queryGraphLine(ctx context.Context, reqBody graphLineRequest) (*graphLineResponse, error) {
	if c.baseURL == "" || c.http == nil {
		return nil, fmt.Errorf("akvorado: client not configured")
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("akvorado: marshal graph/line request: %w", err)
	}

	const path = "/api/v0/console/graph/line"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("akvorado: build request %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("akvorado: POST %s failed: %w", path, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("akvorado: read %s response: %w", path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("akvorado: POST %s HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out graphLineResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("akvorado: decode %s response: %w", path, err)
	}
	return &out, nil
}

// GetFlowSummary returns flow stats for the given interface over the last 5 minutes.
func (c *Client) GetFlowSummary(ctx context.Context, iface string) (*FlowSummary, error) {
	trimmed := sanitizeFilterValue(iface)
	if trimmed == "" {
		return nil, fmt.Errorf("akvorado: interface is required")
	}

	end := time.Now().UTC()
	start := end.Add(-5 * time.Minute)
	res, err := c.queryGraphLine(ctx, graphLineRequest{
		Start:      start.Format(time.RFC3339),
		End:        end.Format(time.RFC3339),
		Dimensions: []string{"SrcAddr"},
		Filter:     fmt.Sprintf(`InIfName = "%s"`, trimmed),
		Limit:      defaultFlowLimit,
		Points:     defaultGraphPoints,
		Units:      "l3bps",
	})
	if err != nil {
		return nil, err
	}

	summary := &FlowSummary{Interface: trimmed}
	for i, row := range res.Rows {
		var bps float64
		if i < len(res.Average) {
			bps = res.Average[i]
		}
		if len(row) > 0 && row[0] == "Other" {
			summary.InBps += bps
			continue
		}
		summary.InBps += bps
		if len(row) > 0 {
			summary.TopTalkers = append(summary.TopTalkers, TopTalker{
				SrcIP: row[0],
				Bps:   bps,
			})
		}
	}
	return summary, nil
}

// QueryFlows queries Akvorado 2.4.0 top flows filtered by source (IP or exporter name), destination, and window.
func (c *Client) QueryFlows(ctx context.Context, src, dst string, duration time.Duration) ([]FlowRecord, error) {
	if duration <= 0 {
		duration = defaultQueryDuration
	}

	end := time.Now().UTC()
	start := end.Add(-duration)
	res, err := c.queryGraphLine(ctx, graphLineRequest{
		Start:      start.Format(time.RFC3339),
		End:        end.Format(time.RFC3339),
		Dimensions: []string{"SrcAddr", "DstAddr"},
		Filter:     buildFilter(src, dst),
		Limit:      defaultFlowLimit,
		Points:     defaultGraphPoints,
		Units:      "l3bps",
	})
	if err != nil {
		return nil, err
	}

	records := make([]FlowRecord, 0, len(res.Rows))
	for i, row := range res.Rows {
		if len(row) < 2 || (row[0] == "Other" && row[1] == "Other") {
			continue
		}
		var bps float64
		if i < len(res.Average) {
			bps = res.Average[i]
		}
		var bytes uint64
		if i < len(res.Total) && res.Total[i] > 0 {
			bytes = uint64(res.Total[i] / 8)
		}
		records = append(records, FlowRecord{
			SrcIP: row[0],
			DstIP: row[1],
			Bps:   bps,
			Bytes: bytes,
		})
	}
	return records, nil
}
