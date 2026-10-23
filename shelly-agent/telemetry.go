package main

import "time"

// telemetry is the prescribed dashboard-facing payload, published on
// agent/<device-id>/status. ObservedAt must represent the actual observation.
type telemetry struct {
	DeviceID   string    `json:"device_id"`
	Output     bool      `json:"output"`
	PowerW     float64   `json:"power_w"`
	EnergyWh   float64   `json:"energy_wh"`
	ObservedAt time.Time `json:"observed_at"`
}
