package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

//go:embed web/*
var assets embed.FS

var deviceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

type telemetry struct {
	DeviceID   string    `json:"device_id"`
	Output     bool      `json:"output"`
	PowerW     float64   `json:"power_w"`
	EnergyWh   float64   `json:"energy_wh"`
	ObservedAt time.Time `json:"observed_at"`
}

type device struct {
	telemetry
	ReceivedAt time.Time `json:"received_at"`
}

type snapshot struct {
	Connected  bool              `json:"connected"`
	Devices    map[string]device `json:"devices"`
	StaleAfter float64           `json:"stale_after_seconds"`
}

type dashboard struct {
	mu         sync.Mutex
	devices    map[string]device
	listeners  map[chan struct{}]struct{}
	connected  bool
	staleAfter time.Duration
	publish    func(context.Context, string, []byte) error
}

func newDashboard(staleAfter time.Duration) *dashboard {
	return &dashboard{devices: make(map[string]device), listeners: make(map[chan struct{}]struct{}), staleAfter: staleAfter}
}

func (d *dashboard) notifyLocked() {
	for ch := range d.listeners {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (d *dashboard) setConnected(connected bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.connected = connected
	d.notifyLocked()
}

func (d *dashboard) ingest(topic string, payload []byte) error {
	parts := strings.Split(topic, "/")
	if len(parts) != 3 || parts[0] != "agent" || parts[2] != "status" || !deviceIDPattern.MatchString(parts[1]) {
		return fmt.Errorf("invalid status topic")
	}
	// Pointers distinguish absent values from valid false/zero readings.
	var input struct {
		DeviceID   string     `json:"device_id"`
		Output     *bool      `json:"output"`
		PowerW     *float64   `json:"power_w"`
		EnergyWh   *float64   `json:"energy_wh"`
		ObservedAt *time.Time `json:"observed_at"`
	}
	if err := json.Unmarshal(payload, &input); err != nil {
		return err
	}
	if input.DeviceID != parts[1] || input.Output == nil || input.PowerW == nil || input.EnergyWh == nil || input.ObservedAt == nil || input.ObservedAt.IsZero() {
		return fmt.Errorf("missing telemetry fields or device ID mismatch")
	}
	if *input.PowerW < 0 || *input.EnergyWh < 0 || math.IsInf(*input.PowerW, 0) || math.IsInf(*input.EnergyWh, 0) {
		return fmt.Errorf("invalid power or energy")
	}
	t := telemetry{input.DeviceID, *input.Output, *input.PowerW, *input.EnergyWh, input.ObservedAt.UTC()}
	d.mu.Lock()
	defer d.mu.Unlock()
	if previous, ok := d.devices[t.DeviceID]; ok && t.ObservedAt.Before(previous.ObservedAt) {
		return nil
	}
	d.devices[t.DeviceID] = device{telemetry: t, ReceivedAt: time.Now().UTC()}
	d.notifyLocked()
	return nil
}

func (d *dashboard) state() snapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	copy := make(map[string]device, len(d.devices))
	for id, dev := range d.devices {
		copy[id] = dev
	}
	return snapshot{d.connected, copy, d.staleAfter.Seconds()}
}

func (d *dashboard) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	updates := make(chan struct{}, 1)
	d.mu.Lock()
	d.listeners[updates] = struct{}{}
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.listeners, updates)
		d.mu.Unlock()
	}()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		data, err := json.Marshal(d.state())
		if err != nil {
			return
		}
		// A slow browser must not hold a writer forever.
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-updates:
		case <-ticker.C:
		}
	}
}

func (d *dashboard) command(w http.ResponseWriter, r *http.Request) {
	// The dashboard is a local tool. Reject cross-origin browser commands.
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
			http.Error(w, "origin rejected", http.StatusForbidden)
			return
		}
	}
	id := r.PathValue("id")
	if !deviceIDPattern.MatchString(id) {
		http.Error(w, "invalid device ID", http.StatusBadRequest)
		return
	}
	var input struct {
		On *bool `json:"on"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input.On == nil {
		http.Error(w, "expected JSON object with boolean on", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "unexpected trailing JSON", http.StatusBadRequest)
		return
	}
	d.mu.Lock()
	_, known := d.devices[id]
	connected := d.connected
	d.mu.Unlock()
	if !known {
		http.Error(w, "unknown device", http.StatusNotFound)
		return
	}
	if !connected {
		http.Error(w, "MQTT disconnected", http.StatusServiceUnavailable)
		return
	}
	payload, _ := json.Marshal(struct {
		On bool `json:"on"`
	}{*input.On})
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := d.publish(ctx, "agent/"+id+"/command", payload); err != nil {
		log.Printf("command publish for %s: %v", id, err)
		http.Error(w, "MQTT publish failed", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = io.WriteString(w, `{"status":"sent"}`)
}

func (d *dashboard) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/events", d.events)
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(d.state())
	})
	mux.HandleFunc("POST /api/devices/{id}/command", d.command)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		data, _ := assets.ReadFile("web/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
	return mux
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func run(ctx context.Context) error {
	staleAfter, err := time.ParseDuration(env("STALE_AFTER", "20s"))
	if err != nil || staleAfter <= 0 {
		return fmt.Errorf("STALE_AFTER must be a positive duration")
	}
	broker := env("MQTT_BROKER", "tcp://localhost:1883")
	u, err := url.Parse(broker)
	if err != nil || u.Scheme != "tcp" || u.Host == "" || u.User != nil {
		return fmt.Errorf("MQTT_BROKER must be a tcp://host:port address")
	}
	d := newDashboard(staleAfter)
	opts := mqtt.NewClientOptions().AddBroker(broker).
		SetClientID(fmt.Sprintf("shelly-ui-%d", time.Now().UnixNano())).
		SetConnectTimeout(3 * time.Second).SetWriteTimeout(3 * time.Second).
		SetConnectRetry(true).SetConnectRetryInterval(2 * time.Second).
		SetAutoReconnect(true).SetMaxReconnectInterval(10 * time.Second)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		for ctx.Err() == nil && c.IsConnectionOpen() {
			token := c.Subscribe("agent/+/status", 1, func(_ mqtt.Client, msg mqtt.Message) {
				if err := d.ingest(msg.Topic(), msg.Payload()); err != nil {
					log.Printf("ignored telemetry on %s: %v", msg.Topic(), err)
				}
			})
			if token.WaitTimeout(3*time.Second) && token.Error() == nil {
				d.setConnected(true)
				log.Print("MQTT connected and subscribed")
				return
			}
			log.Print("MQTT subscription failed; retrying")
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		d.setConnected(false)
		log.Printf("MQTT connection lost: %v", err)
	})
	client := mqtt.NewClient(opts)
	d.publish = func(ctx context.Context, topic string, payload []byte) error {
		if !client.IsConnectionOpen() {
			return fmt.Errorf("MQTT disconnected")
		}
		token := client.Publish(topic, 1, false, payload)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-token.Done():
			return token.Error()
		}
	}
	client.Connect() // Connection retries do not block HTTP startup.
	defer client.Disconnect(250)
	server := &http.Server{
		Addr: env("HTTP_ADDR", ":8090"), Handler: d.handler(),
		ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("dashboard listening on %s", server.Addr)
		errCh <- server.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}
		return nil
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Printf("dashboard failed: %v", err)
		os.Exit(1)
	}
}
