// Package enricher augments inbound events with LibreNMS and Akvorado context.
package enricher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/akvorado"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/eventbus"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/librenms"
)

const (
	defaultTimeout    = 5 * time.Second
	defaultFlowWindow = 5 * time.Minute
)

// Enricher queries LibreNMS and Akvorado in parallel under a hard timeout.
type Enricher struct {
	librenms librenms.LibreNMSClient
	akvorado akvorado.AkvoradoClient
	timeout  time.Duration
	logger   *slog.Logger
}

// New creates an Enricher. Either client may be nil (that source is skipped).
func New(lnms librenms.LibreNMSClient, akv akvorado.AkvoradoClient, timeout time.Duration, logger *slog.Logger) *Enricher {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Enricher{
		librenms: lnms,
		akvorado: akv,
		timeout:  timeout,
		logger:   logger,
	}
}

// Enrich populates ev.Device and ev.Flows in parallel, tolerating partial failures.
func (e *Enricher) Enrich(ctx context.Context, ev *eventbus.Event) {
	if ev == nil {
		return
	}

	enrichCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)

	if e.librenms != nil && ev.Host != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dev, err := e.librenms.GetDevice(enrichCtx, ev.Host)
			if err != nil {
				e.logger.Warn("enricher: librenms GetDevice failed", "host", ev.Host, "error", err)
				return
			}
			mu.Lock()
			ev.Device = dev
			mu.Unlock()
		}()
	}

	if e.akvorado != nil && ev.Host != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			flows, err := e.akvorado.QueryFlows(enrichCtx, ev.Host, "", defaultFlowWindow)
			if err != nil {
				e.logger.Warn("enricher: akvorado QueryFlows failed", "host", ev.Host, "error", err)
				return
			}
			mu.Lock()
			ev.Flows = flows
			mu.Unlock()
		}()
	}

	wg.Wait()
}
