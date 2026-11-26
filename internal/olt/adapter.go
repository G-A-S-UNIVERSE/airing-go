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

// BaseOLTAdapter is the contract every vendor adapter must satisfy.
type BaseOLTAdapter interface {
	VendorKey() string
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	ListONUs(ctx context.Context) ([]ONUStatus, error)
	GetONU(ctx context.Context, onuID string) (*ONUStatus, error)
	RebootONU(ctx context.Context, onuID string) error
}
