// Package akvorado provides a client for the Akvorado flow analytics API.
// Akvorado is airing-go's traffic intelligence layer: NetFlow/sFlow anomaly
// detection, top-talker analysis, and upstream saturation alerts.
package akvorado

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Client communicates with an Akvorado instance.
type Client struct {
	baseURL string
	http    *http.Client
}

// Config holds connection parameters.
type Config struct {
	BaseURL string // e.g. "http://akvorado.example.com"
	Timeout time.Duration
}

// New creates an Akvorado client.
func New(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &Client{
		baseURL: cfg.BaseURL,
		http:    &http.Client{Timeout: cfg.Timeout},
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

// GetFlowSummary returns flow stats for the given interface over the last window.
func (c *Client) GetFlowSummary(_ context.Context, iface string) (*FlowSummary, error) {
	if c.baseURL == "" || c.http == nil {
		return nil, fmt.Errorf("akvorado: client not configured")
	}
	return nil, fmt.Errorf("akvorado: GetFlowSummary(%s) not yet implemented for %s", iface, c.baseURL)
}
