package main

import (
	"encoding/json"
	"os"
	"strconv"
)

// Config holds all runtime settings for the simulator.
type Config struct {
	DeviceID  string `json:"device_id"`
	HTTPPort  int    `json:"http_port"`
	HTTPSPort int    `json:"https_port"`
	// Devices optionally configures multiple simulated plugs, each on its
	// own HTTP(S) port (mirroring real devices being separate network
	// endpoints). When empty, a single device is derived from DeviceID/
	// HTTPPort/HTTPSPort above.
	Devices []DeviceConfig `json:"devices"`
	TLS     struct {
		Enabled  bool   `json:"enabled"`
		CertFile string `json:"cert_file"`
		KeyFile  string `json:"key_file"`
	} `json:"tls"`
	Auth struct {
		Enabled  bool   `json:"enabled"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"auth"`
	MQTT struct {
		Broker   string `json:"broker"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"mqtt"`
}

// DeviceConfig describes a single simulated plug.
type DeviceConfig struct {
	ID        string `json:"id"`
	HTTPPort  int    `json:"http_port"`
	HTTPSPort int    `json:"https_port"`
}

func defaultConfig() Config {
	var c Config
	c.DeviceID = "shellyplug-s-SIM01"
	c.HTTPPort = 8080
	c.HTTPSPort = 8443
	c.MQTT.Broker = "tcp://localhost:1883"
	return c
}

// loadConfig builds the config from defaults, an optional JSON file
// (path from --config arg or CONFIG_FILE env), then env var overrides.
func loadConfig(args []string) (Config, error) {
	c := defaultConfig()

	path := os.Getenv("CONFIG_FILE")
	for i, a := range args {
		if a == "--config" && i+1 < len(args) {
			path = args[i+1]
		}
	}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return c, err
		}
		if err := json.Unmarshal(data, &c); err != nil {
			return c, err
		}
	}

	applyEnvOverrides(&c)
	return c, nil
}

// devices returns the configured devices, falling back to a single device
// built from the top-level DeviceID/HTTPPort/HTTPSPort fields when the
// "devices" list is empty (keeps single-device/env-var mode working as before).
func (c Config) devices() []DeviceConfig {
	if len(c.Devices) > 0 {
		return c.Devices
	}
	return []DeviceConfig{{ID: c.DeviceID, HTTPPort: c.HTTPPort, HTTPSPort: c.HTTPSPort}}
}

func applyEnvOverrides(c *Config) {
	if v := os.Getenv("DEVICE_ID"); v != "" {
		c.DeviceID = v
	}
	if v := os.Getenv("HTTP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.HTTPPort = p
		}
	}
	if v := os.Getenv("HTTPS_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.HTTPSPort = p
		}
	}
	if v := os.Getenv("TLS_ENABLED"); v != "" {
		c.TLS.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("TLS_CERT_FILE"); v != "" {
		c.TLS.CertFile = v
	}
	if v := os.Getenv("TLS_KEY_FILE"); v != "" {
		c.TLS.KeyFile = v
	}
	if v := os.Getenv("AUTH_ENABLED"); v != "" {
		c.Auth.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("AUTH_USERNAME"); v != "" {
		c.Auth.Username = v
	}
	if v := os.Getenv("AUTH_PASSWORD"); v != "" {
		c.Auth.Password = v
	}
	if v := os.Getenv("MQTT_BROKER"); v != "" {
		c.MQTT.Broker = v
	}
	if v := os.Getenv("MQTT_USERNAME"); v != "" {
		c.MQTT.Username = v
	}
	if v := os.Getenv("MQTT_PASSWORD"); v != "" {
		c.MQTT.Password = v
	}
}
