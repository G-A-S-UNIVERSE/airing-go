// Package librenms provides a client for the LibreNMS REST API (/api/v0).
// LibreNMS is used as enrichment context: device inventory, port status,
// and bandwidth baselines that augment Zabbix event data.
package librenms

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

const defaultTimeout = 15 * time.Second

// Client communicates with a LibreNMS server.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// Config holds connection parameters.
type Config struct {
	BaseURL    string // e.g. "http://librenms.example.com"
	Token      string // API token from Settings -> API Access
	Timeout    time.Duration
	HTTPClient *http.Client
}

// New creates a LibreNMS client.
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
		token:   cfg.Token,
		http:    hc,
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

type devicesResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Devices []Device `json:"devices"`
}

func (c *Client) doGet(ctx context.Context, path string, out any) error {
	if c.baseURL == "" || c.token == "" || c.http == nil {
		return fmt.Errorf("librenms: client not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("librenms: build request %s: %w", path, err)
	}
	req.Header.Set("X-Auth-Token", c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("librenms: GET %s failed: %w", path, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("librenms: read %s response: %w", path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("librenms: GET %s HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("librenms: decode %s response: %w", path, err)
	}
	return nil
}

// GetDevices returns all monitored devices.
func (c *Client) GetDevices(ctx context.Context) ([]Device, error) {
	var res devicesResponse
	if err := c.doGet(ctx, "/api/v0/devices", &res); err != nil {
		return nil, err
	}
	if res.Status != "" && res.Status != "ok" {
		return nil, fmt.Errorf("librenms: GetDevices error status %q: %s", res.Status, res.Message)
	}
	return res.Devices, nil
}

// ListDevices is an alias for GetDevices.
func (c *Client) ListDevices(ctx context.Context) ([]Device, error) {
	return c.GetDevices(ctx)
}

// GetDevice returns a single device by hostname.
func (c *Client) GetDevice(ctx context.Context, hostname string) (*Device, error) {
	trimmed := strings.TrimSpace(hostname)
	if trimmed == "" {
		return nil, fmt.Errorf("librenms: hostname is required")
	}

	var res devicesResponse
	path := "/api/v0/devices/" + url.PathEscape(trimmed)
	if err := c.doGet(ctx, path, &res); err != nil {
		return nil, err
	}
	if res.Status != "" && res.Status != "ok" {
		return nil, fmt.Errorf("librenms: GetDevice error status %q: %s", res.Status, res.Message)
	}
	if len(res.Devices) == 0 {
		return nil, fmt.Errorf("librenms: device %q not found", trimmed)
	}
	d := res.Devices[0]
	return &d, nil
}
