package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/olt"
	"github.com/G-A-S-UNIVERSE/airing-go/internal/olt/zte"
)

const oltCacheTTL = 60 * time.Second

// ConfiguredOLT describes a managed OLT loaded from AIRING_OLTS_JSON.
type ConfiguredOLT struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Vendor     string `json:"vendor"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	TimeoutSec int    `json:"timeout_sec"`
}

type cachedSummary struct {
	data      *olt.OLTSummary
	expiresAt time.Time
}

type cachedONUs struct {
	data      []olt.ONUStatus
	expiresAt time.Time
}

// OLTManager caches read-only telemetry from configured OLT instances.
type OLTManager struct {
	mu       sync.RWMutex
	units    map[string]ConfiguredOLT
	order    []string
	adapters map[string]olt.BaseOLTAdapter
	sumCache map[string]cachedSummary
	onuCache map[string]cachedONUs
}

// NewOLTManager parses AIRING_OLTS_JSON; empty string yields an empty manager.
func NewOLTManager(rawJSON string) (*OLTManager, error) {
	mgr := &OLTManager{
		units:    make(map[string]ConfiguredOLT),
		adapters: make(map[string]olt.BaseOLTAdapter),
		sumCache: make(map[string]cachedSummary),
		onuCache: make(map[string]cachedONUs),
	}
	trimmed := strings.TrimSpace(rawJSON)
	if trimmed == "" {
		return mgr, nil
	}

	var list []ConfiguredOLT
	if err := json.Unmarshal([]byte(trimmed), &list); err != nil {
		return nil, fmt.Errorf("parse AIRING_OLTS_JSON: %w", err)
	}
	for _, cfg := range list {
		key := strings.TrimSpace(cfg.Key)
		if key == "" {
			continue
		}
		cfg.Key = key
		if cfg.Name == "" {
			cfg.Name = key
		}
		if cfg.Vendor == "" {
			cfg.Vendor = "zte_c320"
		}
		if cfg.Port <= 0 {
			cfg.Port = 23
		}
		if cfg.TimeoutSec <= 0 {
			cfg.TimeoutSec = 15
		}
		adapter, err := buildAdapterForConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("olt %q: %w", key, err)
		}
		mgr.units[key] = cfg
		mgr.order = append(mgr.order, key)
		mgr.adapters[key] = adapter
	}
	return mgr, nil
}

func buildAdapterForConfig(cfg ConfiguredOLT) (olt.BaseOLTAdapter, error) {
	switch cfg.Vendor {
	case "zte_c320":
		return zte.New(zte.Config{
			Host:     cfg.Host,
			Port:     cfg.Port,
			Username: cfg.Username,
			Password: cfg.Password,
			Timeout:  time.Duration(cfg.TimeoutSec) * time.Second,
		}), nil
	default:
		return olt.New(cfg.Vendor)
	}
}

func (m *OLTManager) Count() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.order)
}

func (m *OLTManager) ListSummaries() []oltSummary {
	out := make([]oltSummary, 0)
	if m != nil {
		m.mu.RLock()
		for _, k := range m.order {
			u := m.units[k]
			out = append(out, oltSummary{
				Key:        u.Key,
				Name:       u.Name,
				Vendor:     u.Vendor,
				Connected:  true,
				Configured: true,
				ReadOnly:   true,
			})
		}
		m.mu.RUnlock()
	}
	keys := olt.Keys()
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, oltSummary{Key: k, Vendor: k, Connected: false, Configured: false, ReadOnly: true})
	}
	return out
}

func (m *OLTManager) getAdapter(key string) (ConfiguredOLT, olt.BaseOLTAdapter, bool) {
	if m == nil {
		return ConfiguredOLT{}, nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok1 := m.units[key]
	ad, ok2 := m.adapters[key]
	return cfg, ad, ok1 && ok2
}

func (m *OLTManager) Summary(ctx context.Context, key string, forceRefresh bool) (*olt.OLTSummary, int, error) {
	cfg, ad, ok := m.getAdapter(key)
	if !ok {
		return nil, http.StatusNotFound, fmt.Errorf("olt %q not configured", key)
	}
	if !forceRefresh {
		m.mu.RLock()
		c, hit := m.sumCache[key]
		m.mu.RUnlock()
		if hit && time.Now().Before(c.expiresAt) && c.data != nil {
			return c.data, http.StatusOK, nil
		}
	}

	sp, ok := ad.(olt.SummaryProvider)
	var sum *olt.OLTSummary
	var err error
	if ok {
		sum, err = sp.GetSummary(ctx)
	} else {
		onus, lerr := ad.ListONUs(ctx)
		err = lerr
		if err == nil {
			sum = &olt.OLTSummary{
				Vendor:    cfg.Vendor,
				TotalONUs: len(onus),
				ReadOnly:  true,
				PolledAt:  time.Now().UTC().Format(time.RFC3339),
			}
		}
	}
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	sum.Key = cfg.Key
	if cfg.Name != "" {
		sum.Name = cfg.Name
	}
	sum.ReadOnly = true

	m.mu.Lock()
	m.sumCache[key] = cachedSummary{data: sum, expiresAt: time.Now().Add(oltCacheTTL)}
	m.mu.Unlock()
	return sum, http.StatusOK, nil
}

func (m *OLTManager) ListONUs(ctx context.Context, key, statusFilter string, forceRefresh bool) ([]olt.ONUStatus, int, error) {
	_, ad, ok := m.getAdapter(key)
	if !ok {
		return nil, http.StatusNotFound, fmt.Errorf("olt %q not configured", key)
	}
	var onus []olt.ONUStatus
	var fromCache bool
	if !forceRefresh {
		m.mu.RLock()
		c, hit := m.onuCache[key]
		m.mu.RUnlock()
		if hit && time.Now().Before(c.expiresAt) {
			onus = c.data
			fromCache = true
		}
	}
	if !fromCache {
		var err error
		onus, err = ad.ListONUs(ctx)
		if err != nil {
			return nil, http.StatusBadGateway, err
		}
		m.mu.Lock()
		m.onuCache[key] = cachedONUs{data: onus, expiresAt: time.Now().Add(oltCacheTTL)}
		m.mu.Unlock()
	}

	f := strings.ToLower(strings.TrimSpace(statusFilter))
	if f == "" || f == "all" {
		return onus, http.StatusOK, nil
	}
	filtered := make([]olt.ONUStatus, 0, len(onus))
	for _, o := range onus {
		if f == "problem" {
			if o.Status != "online" {
				filtered = append(filtered, o)
			}
		} else if strings.EqualFold(o.Status, f) {
			filtered = append(filtered, o)
		}
	}
	return filtered, http.StatusOK, nil
}

func (m *OLTManager) GetONU(ctx context.Context, key, onuID string) (*olt.ONUStatus, int, error) {
	_, ad, ok := m.getAdapter(key)
	if !ok {
		return nil, http.StatusNotFound, fmt.Errorf("olt %q not configured", key)
	}
	st, err := ad.GetONU(ctx, onuID)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	return st, http.StatusOK, nil
}

func (m *OLTManager) RebootONU(ctx context.Context, key, onuID string) (int, error) {
	if _, ad, ok := m.getAdapter(key); ok {
		if err := ad.RebootONU(ctx, onuID); err != nil {
			return http.StatusBadGateway, err
		}
		return http.StatusOK, nil
	}
	ad, err := olt.New(key)
	if err != nil {
		return http.StatusNotFound, err
	}
	if err := ad.RebootONU(ctx, onuID); err != nil {
		return http.StatusBadGateway, err
	}
	return http.StatusOK, nil
}
