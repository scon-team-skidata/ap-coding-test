package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Decode tokens explicitly to reject duplicate keys as well as unknown fields.
func parseCommand(payload []byte) (bool, error) {
	if len(payload) > 1024 {
		return false, fmt.Errorf("command exceeds 1024 bytes")
	}
	d := json.NewDecoder(bytes.NewReader(payload))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return false, fmt.Errorf("command must be a JSON object")
	}
	var on bool
	seen := false
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return false, fmt.Errorf("invalid command field: %w", err)
		}
		if key != "on" {
			return false, fmt.Errorf("unsupported field %q", key)
		}
		if seen {
			return false, fmt.Errorf("duplicate on field")
		}
		value, err := d.Token()
		if err != nil {
			return false, fmt.Errorf("invalid on field: %w", err)
		}
		var ok bool
		on, ok = value.(bool)
		if !ok {
			return false, fmt.Errorf("on must be a boolean")
		}
		seen = true
	}
	if _, err := d.Token(); err != nil {
		return false, fmt.Errorf("invalid command object: %w", err)
	}
	if !seen {
		return false, fmt.Errorf("on is required")
	}
	if _, err := d.Token(); err != io.EOF {
		return false, fmt.Errorf("unexpected data after command")
	}
	return on, nil
}
