// Package rules maps enriched events into deterministic operational actions.
package rules

import (
	"fmt"
	"strings"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/eventbus"
)

const defaultOLTKey = "zte_c320"

// Action is the rule evaluation output for an event.
type Action struct {
	Type    string // "reboot_onu" | "notify_only" | "fetch_onus"
	OLTKey  string
	ONUID   string
	Message string
}

// Engine evaluates events into Actions.
type Engine struct{}

// New creates a RuleEngine.
func New() *Engine {
	return &Engine{}
}

// Evaluate maps an enriched Event into a single Action.
func (e *Engine) Evaluate(ev eventbus.Event) Action {
	if strings.EqualFold(ev.Status, "RESOLVED") {
		return Action{
			Type:    "notify_only",
			Message: fmt.Sprintf("[%s] RESOLVED on %s", ev.Name, ev.Host),
		}
	}

	kind := strings.ToUpper(strings.TrimSpace(ev.Tags["type"]))
	if kind == "" {
		kind = strings.ToUpper(strings.TrimSpace(ev.Name))
	}

	switch {
	case strings.Contains(kind, "OLT_DOWN"):
		oltKey := strings.TrimSpace(ev.Tags["olt_key"])
		if oltKey == "" {
			oltKey = defaultOLTKey
		}
		return Action{
			Type:    "fetch_onus",
			OLTKey:  oltKey,
			Message: fmt.Sprintf("OLT_DOWN on %s: inspecting ONUs via %s", ev.Host, oltKey),
		}
	case strings.Contains(kind, "HIGH_BW"):
		return Action{
			Type:    "notify_only",
			Message: fmt.Sprintf("HIGH_BW on %s (%d flow records)", ev.Host, len(ev.Flows)),
		}
	default:
		return Action{
			Type:    "notify_only",
			Message: fmt.Sprintf("[%s] %s on %s", ev.Severity, ev.Name, ev.Host),
		}
	}
}
