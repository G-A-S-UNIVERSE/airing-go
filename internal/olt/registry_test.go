package olt_test

import (
	"context"
	"slices"
	"testing"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/olt"
)

type stubAdapter struct {
	key string
}

func (s *stubAdapter) VendorKey() string                                          { return s.key }
func (s *stubAdapter) Connect(_ context.Context) error                            { return nil }
func (s *stubAdapter) Disconnect(_ context.Context) error                         { return nil }
func (s *stubAdapter) ListONUs(_ context.Context) ([]olt.ONUStatus, error)        { return nil, nil }
func (s *stubAdapter) GetONU(_ context.Context, _ string) (*olt.ONUStatus, error) { return nil, nil }
func (s *stubAdapter) RebootONU(_ context.Context, _ string) error                { return nil }

func TestRegistryRegisterAndLookup(t *testing.T) {
	const testKey = "stub_test_vendor"
	olt.Register(testKey, func() olt.BaseOLTAdapter {
		return &stubAdapter{key: testKey}
	})

	adapter, err := olt.New(testKey)
	if err != nil {
		t.Fatalf("expected registered adapter, got error: %v", err)
	}
	if adapter.VendorKey() != testKey {
		t.Fatalf("expected VendorKey %q, got %q", testKey, adapter.VendorKey())
	}

	if !slices.Contains(olt.Keys(), testKey) {
		t.Fatalf("expected Keys() to contain %q, got %v", testKey, olt.Keys())
	}

	if _, err := olt.New("nonexistent_vendor"); err == nil {
		t.Fatal("expected error for unknown vendor key, got nil")
	}
}
