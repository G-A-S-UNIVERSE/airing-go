// Package zabbix provides a client for the Zabbix JSON-RPC and REST API.
// Zabbix 8 is airing-go's primary event spine: triggers, problems,
// CEP correlations, and acknowledgements all flow through here.
package zabbix

import (
	"context"
	"fmt"
)

// Client communicates with a Zabbix server.
type Client struct {
	url   string
	token string
}

// Config holds connection parameters.
type Config struct {
	URL   string // e.g. "http://zabbix.example.com"
	Token string // API token (Zabbix >= 5.4)
}

// New creates a Zabbix client.
func New(cfg Config) *Client {
	return &Client{url: cfg.URL, token: cfg.Token}
}

// Trigger represents a Zabbix trigger / problem.
type Trigger struct {
	TriggerID   string `json:"triggerid"`
	Description string `json:"description"`
	Priority    string `json:"priority"` // "0"=not classified ... "5"=disaster
	Value       string `json:"value"`    // "0"=OK "1"=PROBLEM
	HostName    string `json:"hostname"`
}

// GetActiveTriggers returns all active (PROBLEM) triggers.
func (c *Client) GetActiveTriggers(_ context.Context) ([]Trigger, error) {
	if c.url == "" || c.token == "" {
		return nil, fmt.Errorf("zabbix: client not configured")
	}
	return nil, fmt.Errorf("zabbix: GetActiveTriggers not yet implemented for %s", c.url)
}

// AcknowledgeProblem acknowledges a problem by event ID.
func (c *Client) AcknowledgeProblem(_ context.Context, eventID, _ string) error {
	if c.url == "" || c.token == "" {
		return fmt.Errorf("zabbix: client not configured")
	}
	return fmt.Errorf("zabbix: AcknowledgeProblem(%s) not yet implemented", eventID)
}
