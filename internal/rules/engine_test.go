package rules_test

import (
	"strings"
	"testing"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/akvorado"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/eventbus"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/rules"
)

func TestEngine_Evaluate(t *testing.T) {
	eng := rules.New()

	t.Run("OLT_DOWN triggers fetch_onus", func(t *testing.T) {
		act := eng.Evaluate(eventbus.Event{
			Name:   "OLT_DOWN",
			Host:   "olt-region1-01",
			Status: "PROBLEM",
			Tags:   map[string]string{"olt_key": "generic_cli"},
		})
		if act.Type != "fetch_onus" || act.OLTKey != "generic_cli" {
			t.Fatalf("unexpected action: %+v", act)
		}
	})

	t.Run("RESOLVED yields notify_only even for OLT_DOWN", func(t *testing.T) {
		act := eng.Evaluate(eventbus.Event{
			Name:   "OLT_DOWN",
			Host:   "olt-region1-01",
			Status: "RESOLVED",
		})
		if act.Type != "notify_only" {
			t.Fatalf("expected notify_only, got %+v", act)
		}
	})

	t.Run("HIGH_BW summarizes flows", func(t *testing.T) {
		act := eng.Evaluate(eventbus.Event{
			Name:   "HIGH_BW interface saturation",
			Host:   "core-sw-01",
			Status: "PROBLEM",
			Flows:  []akvorado.Flow{{SrcIP: "192.0.2.1"}, {SrcIP: "192.0.2.2"}},
		})
		if act.Type != "notify_only" || !strings.Contains(act.Message, "2 flow records") {
			t.Fatalf("unexpected action: %+v", act)
		}
	})

	t.Run("unknown event yields notify_only", func(t *testing.T) {
		act := eng.Evaluate(eventbus.Event{
			Name:     "CUSTOM_TRAP",
			Host:     "sw-02",
			Severity: "warning",
			Status:   "PROBLEM",
		})
		if act.Type != "notify_only" {
			t.Fatalf("expected notify_only, got %+v", act)
		}
	})
}
