// Package notify defines the Notifier interface.
// External systems (CRM chatbox, Slack, PagerDuty) implement this in
// private deployment code — airing-go never imports any concrete notifier.
package notify

import "context"

// Severity of an event.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Event is the envelope sent to all registered notifiers.
type Event struct {
	Kind     string            `json:"kind"` // e.g. "onu.offline" "zabbix.trigger" "traffic.anomaly"
	Severity Severity          `json:"severity"`
	Source   string            `json:"source"` // "olt/zte_c320" | "zabbix" | "akvorado" | "librenms"
	Message  string            `json:"message"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Notifier dispatches airing events to an external system.
// Implementations must be safe for concurrent use.
type Notifier interface {
	Name() string
	Notify(ctx context.Context, event Event) error
}
