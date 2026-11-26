// Package librenms provides a client for the LibreNMS REST API (/api/v0).
// LibreNMS is used as enrichment context: device inventory, port status,
// and bandwidth baselines that augment Zabbix event data.
package librenms

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Client communicates with a LibreNMS server.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// Config holds connection parameters.
type Config struct {
	BaseURL string // e.g. "http://librenms.example.com"
	Token   string // API token from Settings -> API Access
	Timeout time.Duration
}

// New creates a LibreNMS client.
func New(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	return &Client{
		baseURL: cfg.BaseURL,
		token:   cfg.Token,
		http:    &http.Client{Timeout: cfg.Timeout},
	}
}

// Device represents a LibreNMS managed device.
type Device struct {
	DeviceID int    `json:"device_id"`
	Hostname string `json:"hostname"`
	SysName  string `json:"sysName"`
	Status   int    `json:"status"` // 1=up 0=down
	OS       string `json:"os"`
	Hardware string `json:"hardware"`
}

// GetDevices returns all monitored devices.
func (c *Client) GetDevices(_ context.Context) ([]Device, error) {
	if c.baseURL == "" || c.token == "" || c.http == nil {
		return nil, fmt.Errorf("librenms: client not configured")
	}
	return nil, fmt.Errorf("librenms: GetDevices not yet implemented for %s", c.baseURL)
}

// GetDevice returns a single device by hostname.
func (c *Client) GetDevice(_ context.Context, hostname string) (*Device, error) {
	if c.baseURL == "" || c.token == "" || c.http == nil {
		return nil, fmt.Errorf("librenms: client not configured")
	}
	return nil, fmt.Errorf("librenms: GetDevice(%s) not yet implemented", hostname)
}
