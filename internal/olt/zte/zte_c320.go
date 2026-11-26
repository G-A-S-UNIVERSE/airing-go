package zte

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/olt"
)

func init() {
	olt.Register("zte_c320", func() olt.BaseOLTAdapter {
		return &ZteC320Adapter{}
	})
}

// ZteC320Adapter manages a ZTE C320 OLT via HTTP management API.
type ZteC320Adapter struct {
	host   string
	port   int
	user   string
	pass   string
	client *http.Client
}

// Config holds ZTE C320 connection parameters.
type Config struct {
	Host     string
	Port     int // default 80
	Username string
	Password string
	Timeout  time.Duration // default 30s
}

// New creates a ZteC320Adapter from config.
func New(cfg Config) *ZteC320Adapter {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Port == 0 {
		cfg.Port = 80
	}
	return &ZteC320Adapter{
		host:   cfg.Host,
		port:   cfg.Port,
		user:   cfg.Username,
		pass:   cfg.Password,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

func (a *ZteC320Adapter) VendorKey() string { return "zte_c320" }

func (a *ZteC320Adapter) Connect(ctx context.Context) error {
	// ZTE C320 is stateless HTTP - validate reachability.
	url := fmt.Sprintf("http://%s:%d", a.host, a.port)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return fmt.Errorf("zte_c320: build request: %w", err)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("zte_c320: connect to %s: %w", url, err)
	}
	resp.Body.Close()
	return nil
}

func (a *ZteC320Adapter) Disconnect(_ context.Context) error {
	return nil // stateless
}

func (a *ZteC320Adapter) ListONUs(ctx context.Context) ([]olt.ONUStatus, error) {
	// TODO: ZTE C320 ONU list via HTTP API
	// Reference: github.com/G-A-S-UNIVERSE/airing src/adapters/zte_c320.py
	return nil, fmt.Errorf("zte_c320: ListONUs not yet implemented")
}

func (a *ZteC320Adapter) GetONU(_ context.Context, onuID string) (*olt.ONUStatus, error) {
	return nil, fmt.Errorf("zte_c320: GetONU(%q) not yet implemented", onuID)
}

func (a *ZteC320Adapter) RebootONU(_ context.Context, onuID string) error {
	return fmt.Errorf("zte_c320: RebootONU(%q) not yet implemented", onuID)
}
