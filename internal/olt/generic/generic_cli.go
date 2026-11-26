package generic

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/olt"
)

func init() {
	olt.Register("generic_cli", func() olt.BaseOLTAdapter {
		return &GenericCLIAdapter{}
	})
}

// GenericCLIAdapter bridges to an HTTP CLI proxy that speaks to the OLT
// via SSH or Telnet and exposes a REST interface.
// Mirrors airing's GenericCLIAdapter (github.com/G-A-S-UNIVERSE/airing).
type GenericCLIAdapter struct {
	apiURL string
	client *http.Client
}

// Config holds GenericCLI connection parameters.
type Config struct {
	APIURL  string        // e.g. "http://10.0.0.5:9090"
	Timeout time.Duration // default 30s
}

// New creates a GenericCLIAdapter.
func New(cfg Config) *GenericCLIAdapter {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &GenericCLIAdapter{
		apiURL: cfg.APIURL,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

func (a *GenericCLIAdapter) VendorKey() string { return "generic_cli" }

func (a *GenericCLIAdapter) Connect(_ context.Context) error {
	if a.apiURL == "" {
		return fmt.Errorf("generic_cli: APIURL is required")
	}
	return nil
}

func (a *GenericCLIAdapter) Disconnect(_ context.Context) error { return nil }

func (a *GenericCLIAdapter) ListONUs(_ context.Context) ([]olt.ONUStatus, error) {
	return nil, fmt.Errorf("generic_cli: ListONUs not yet implemented")
}

func (a *GenericCLIAdapter) GetONU(_ context.Context, onuID string) (*olt.ONUStatus, error) {
	return nil, fmt.Errorf("generic_cli: GetONU(%q) not yet implemented", onuID)
}

func (a *GenericCLIAdapter) RebootONU(_ context.Context, onuID string) error {
	return fmt.Errorf("generic_cli: RebootONU(%q) not yet implemented", onuID)
}
