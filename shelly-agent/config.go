package main

import (
	"errors"
	"time"
)

// config describes the core environment settings. See .env.example for host
// values; Compose supplies container addresses instead.
type config struct {
	shellyURL    string        // SHELLY_URL
	broker       string        // MQTT_BROKER
	pollInterval time.Duration // POLL_INTERVAL
	httpTimeout  time.Duration // HTTP_TIMEOUT
}

// loadConfig accepts an environment lookup function, e.g. os.Getenv, so tests
// can supply isolated settings without changing process environment.
func loadConfig(getenv func(string) string) (config, error) {
	// TODO: Choose/document defaults, parse durations, and validate settings.
	return config{}, errors.New("TODO: implement configuration parsing")
}
