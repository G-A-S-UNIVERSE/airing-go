package olt

import "context"

// ONUStatus represents the operational state of an ONU/ONT device.
type ONUStatus struct {
	ONUID        string  `json:"onu_id"`
	SerialNumber string  `json:"serial_number"`
	Status       string  `json:"status"` // "online" | "offline" | "unknown"
	RxPower      float64 `json:"rx_power_dbm"`
	TxPower      float64 `json:"tx_power_dbm"`
}

// OLTDevice describes a managed OLT unit.
type OLTDevice struct {
	ID     string `json:"id"`
	Vendor string `json:"vendor"`
	Model  string `json:"model"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
}

// OLTSummary holds read-only health & ONU aggregate telemetry for one OLT.
type OLTSummary struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	Vendor         string `json:"vendor"`
	Model          string `json:"model,omitempty"`
	Firmware       string `json:"firmware,omitempty"`
	SystemName     string `json:"system_name,omitempty"`
	Location       string `json:"location,omitempty"`
	Uptime         string `json:"uptime,omitempty"`
	CardsTotal     int    `json:"cards_total"`
	CardsInService int    `json:"cards_in_service"`
	TotalONUs      int    `json:"total_onus"`
	OnlineONUs     int    `json:"online_onus"`
	OfflineONUs    int    `json:"offline_onus"`
	LosONUs        int    `json:"los_onus"`
	DyingGaspONUs  int    `json:"dying_gasp_onus"`
	ReadOnly       bool   `json:"read_only"`
	PolledAt       string `json:"polled_at"`
}

// SummaryProvider is optionally implemented by adapters supporting read-only summary polling.
type SummaryProvider interface {
	GetSummary(ctx context.Context) (*OLTSummary, error)
}

// BaseOLTAdapter is the contract every vendor adapter must satisfy.
type BaseOLTAdapter interface {
	VendorKey() string
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	ListONUs(ctx context.Context) ([]ONUStatus, error)
	GetONU(ctx context.Context, onuID string) (*ONUStatus, error)
	RebootONU(ctx context.Context, onuID string) error
}
