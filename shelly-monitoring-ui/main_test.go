package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func reading(id, observed string) string {
	return `{"device_id":"` + id + `","output":false,"power_w":0,"energy_wh":1.5,"observed_at":"` + observed + `"}`
}

func TestTelemetryValidationAndOrdering(t *testing.T) {
	d := newDashboard(20 * time.Second)
	for _, tt := range []struct{ topic, payload string }{
		{"agent/plug/status", reading("other", "2026-01-01T00:00:00Z")},
		{"agent/plug/command", reading("plug", "2026-01-01T00:00:00Z")},
		{"agent/plug/status", `{"device_id":"plug"}`},
		{"agent/plug/status", `{not json}`},
		{"agent/plug/status", strings.Replace(reading("plug", "2026-01-01T00:00:00Z"), `"power_w":0`, `"power_w":-1`, 1)},
	} {
		if err := d.ingest(tt.topic, []byte(tt.payload)); err == nil {
			t.Errorf("accepted invalid telemetry: %s %s", tt.topic, tt.payload)
		}
	}
	if len(d.state().Devices) != 0 {
		t.Fatal("invalid telemetry changed state")
	}
	for _, observed := range []string{"2026-01-02T00:00:00Z", "2026-01-01T00:00:00Z"} {
		if err := d.ingest("agent/plug/status", []byte(reading("plug", observed))); err != nil {
			t.Fatal(err)
		}
	}
	dev := d.state().Devices["plug"]
	if dev.Output || dev.PowerW != 0 || dev.ObservedAt.Day() != 2 {
		t.Fatalf("wrong latest reading: %+v", dev)
	}
}

func TestCommandPublishing(t *testing.T) {
	for _, tt := range []struct {
		name, body, path, origin string
		connected, fail          bool
		want                     int
	}{
		{name: "on", body: `{"on":true}`, connected: true, want: 202},
		{name: "off", body: `{"on":false}`, connected: true, want: 202},
		{name: "missing", body: `{}`, connected: true, want: 400},
		{name: "null", body: `{"on":null}`, connected: true, want: 400},
		{name: "wrong type", body: `{"on":"false"}`, connected: true, want: 400},
		{name: "extra field", body: `{"on":true,"toggle":true}`, connected: true, want: 400},
		{name: "trailing JSON", body: `{"on":true}{}`, connected: true, want: 400},
		{name: "offline", body: `{"on":true}`, want: 503},
		{name: "unknown", path: "unknown", body: `{"on":true}`, connected: true, want: 404},
		{name: "publish failure", body: `{"on":true}`, connected: true, fail: true, want: 502},
		{name: "cross origin", body: `{"on":true}`, connected: true, origin: "https://other.example", want: 403},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := newDashboard(20 * time.Second)
			if err := d.ingest("agent/plug/status", []byte(reading("plug", "2026-01-01T00:00:00Z"))); err != nil {
				t.Fatal(err)
			}
			d.setConnected(tt.connected)
			calls := 0
			d.publish = func(_ context.Context, topic string, payload []byte) error {
				calls++
				if topic != "agent/plug/command" || string(payload) != tt.body {
					t.Errorf("wrong command: %s %s", topic, payload)
				}
				if tt.fail {
					return errors.New("broker unavailable")
				}
				return nil
			}
			id := tt.path
			if id == "" {
				id = "plug"
			}
			r := httptest.NewRequest("POST", "/api/devices/"+id+"/command", strings.NewReader(tt.body))
			r.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()
			d.handler().ServeHTTP(w, r)
			if w.Code != tt.want {
				t.Fatalf("got %d, want %d: %s", w.Code, tt.want, w.Body)
			}
			if (calls > 0) != (tt.want == 202 || tt.fail) {
				t.Fatalf("unexpected publish count %d", calls)
			}
			if d.state().Devices["plug"].Output {
				t.Fatal("command changed observed state without telemetry")
			}
		})
	}
}

func TestEventsInitialSnapshotAndUpdate(t *testing.T) {
	d := newDashboard(20 * time.Second)
	server := httptest.NewServer(d.handler())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/events", nil)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	readState := func() snapshot {
		t.Helper()
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		var state snapshot
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &state); err != nil {
			t.Fatal(err)
		}
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatal(err)
		}
		return state
	}
	if len(readState().Devices) != 0 {
		t.Fatal("expected empty initial snapshot")
	}
	if err := d.ingest("agent/plug/status", []byte(reading("plug", "2026-01-01T00:00:00Z"))); err != nil {
		t.Fatal(err)
	}
	if _, ok := readState().Devices["plug"]; !ok {
		t.Fatal("stream did not deliver telemetry update")
	}
}
