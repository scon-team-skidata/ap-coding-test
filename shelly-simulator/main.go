package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// runningDevice holds everything that needs clean shutdown for one
// simulated device.
type runningDevice struct {
	cancel context.CancelFunc
	http   *http.Server
	https  *http.Server
	mqtt   mqtt.Client
}

func main() {
	cfg, err := loadConfig(os.Args[1:])
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	var cert tls.Certificate
	if cfg.TLS.Enabled {
		cert, err = loadOrGenerateCert(cfg)
		if err != nil {
			log.Fatalf("tls cert: %v", err)
		}
	}

	var running []*runningDevice
	for _, dev := range cfg.devices() {
		running = append(running, startDevice(cfg, dev, cert))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("shutting down")
	for _, rd := range running {
		rd.cancel()
		_ = rd.http.Shutdown(context.Background())
		if rd.https != nil {
			_ = rd.https.Shutdown(context.Background())
		}
		if rd.mqtt != nil {
			rd.mqtt.Disconnect(250)
		}
	}
}

// startDevice wires up one simulated plug: its own State, HTTP(S) servers,
// and its own MQTT client (each real device is a separate MQTT client too).
func startDevice(cfg Config, dev DeviceConfig, cert tls.Certificate) *runningDevice {
	st := newState(dev.ID, cfg.Auth.Enabled)

	mux := http.NewServeMux()
	registerGen1(mux, st)
	registerGen2HTTP(mux, st)
	registerWSRPC(mux, st)
	handler := withBasicAuth(cfg, mux)

	rd := &runningDevice{}

	rd.http = &http.Server{Addr: fmt.Sprintf(":%d", dev.HTTPPort), Handler: handler}
	go func() {
		log.Printf("[%s] HTTP listening on %s", dev.ID, rd.http.Addr)
		if err := rd.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[%s] http server: %v", dev.ID, err)
		}
	}()

	if cfg.TLS.Enabled {
		rd.https = &http.Server{
			Addr:      fmt.Sprintf(":%d", dev.HTTPSPort),
			Handler:   handler,
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		}
		go func() {
			log.Printf("[%s] HTTPS listening on %s", dev.ID, rd.https.Addr)
			if err := rd.https.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Fatalf("[%s] https server: %v", dev.ID, err)
			}
		}()
	}

	client, err := startMQTT(cfg, st)
	if err != nil {
		log.Printf("[%s] mqtt connect failed (continuing without MQTT): %v", dev.ID, err)
	}
	rd.mqtt = client
	ctx, cancel := context.WithCancel(context.Background())
	rd.cancel = cancel
	go deviceTicker(ctx, client, st, 5*time.Second)

	return rd
}
