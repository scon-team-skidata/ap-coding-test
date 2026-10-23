# Shelly simulator

Go smart-plug simulator used by the [coding-test environment](../README.md).
It exposes Gen1 HTTP endpoints, Gen2 HTTP/WebSocket RPC, and native MQTT topics
against shared relay and power/energy state.

## Shared environment

From the repository root:

```sh
make up
```

To start just the simulator and its MQTT broker:

```sh
docker compose up --build simulator
```

This command runs in the foreground; add `-d` to run in the background. From
`shelly-simulator/`, use the explicit root Compose path instead:

```sh
docker compose -f ../docker-compose.yml up --build -d simulator
```

There is no simulator-specific Compose stack. Root `make down` stops the shared
environment; `docker compose stop simulator` from the root stops only this service.

The root Compose file owns service configuration and the broker setup in
`infrastructure/mosquitto/`. By default it uses HTTP without authentication or
TLS. MQTT is anonymous and unencrypted in the local environment.

```sh
curl -s http://localhost:8081/status
curl -s 'http://localhost:8081/relay/0?turn=on'
```

For optional HTTPS and HTTP Basic auth, run `make up-secure` from the root. The
override `docker-compose.secure.yml` enables HTTPS on host port 8443 and auth on
both API listeners, using `admin` / `admin` unless overridden in root `.env`.
The generated self-signed certificate requires `curl -k` locally. Configure
`TLS_CERT_FILE` and `TLS_KEY_FILE` to use an existing certificate and key instead.
Run root `make up` to return to HTTP without authentication/TLS.

## Local development

Run Go commands from this directory:

```sh
make build
make test
make vet
make run
```

Without overrides, the simulator uses HTTP port 8080, device ID
`shellyplug-s-SIM01`, and MQTT broker `tcp://localhost:1883`; TLS and API
authentication are disabled. Port 8080 is also the Docker dashboard's host port.
To replace just the Docker simulator while keeping the broker/UI running, use
host port 8081 (commands from `shelly-simulator/`):

```sh
docker compose -f ../docker-compose.yml stop simulator
HTTP_PORT=8081 make run
```

Local agents can then use `http://localhost:8081`. A containerized agent still
points at the Compose service, so this setup is for running the agent locally too.
The process continues serving HTTP and simulating measurements if its initial
MQTT connection fails; restart it after making the broker available to enable
native MQTT. Native subscriptions are not automatically restored after a broker
restart; restart the simulator if native commands stop working.

Example local run with the optional secure API settings:

```sh
HTTP_PORT=8081 TLS_ENABLED=true AUTH_ENABLED=true AUTH_USERNAME=admin AUTH_PASSWORD=admin \
  MQTT_BROKER=tcp://localhost:1883 go run .
```

### Configuration

Configuration is loaded from defaults, then an optional JSON file specified by
`--config` or `CONFIG_FILE`, then environment overrides:

| Variable | Purpose |
| --- | --- |
| `DEVICE_ID` | Single-device ID |
| `HTTP_PORT`, `HTTPS_PORT` | Single-device listen ports |
| `TLS_ENABLED` | Enable HTTPS (`true` or `1`) |
| `TLS_CERT_FILE`, `TLS_KEY_FILE` | Optional server certificate/key paths |
| `AUTH_ENABLED` | Enable HTTP Basic authentication (`true` or `1`) |
| `AUTH_USERNAME`, `AUTH_PASSWORD` | API credentials |
| `MQTT_BROKER` | MQTT broker URI |
| `MQTT_USERNAME`, `MQTT_PASSWORD` | Optional MQTT credentials |

`config.example.json` demonstrates two devices on ports 8080/8443 and 8081/8444:

```sh
TLS_ENABLED=true AUTH_ENABLED=true AUTH_PASSWORD=admin \
  go run . --config config.example.json
```

The example file itself disables TLS and authentication; the command above
explicitly enables both. A nonempty `devices` list supplies IDs and ports for
each device instead of the top-level single-device settings. When using this
configuration inside Docker, mount it, set `CONFIG_FILE`, and publish the
additional ports in the root Compose file.
The example's ports 8080/8081 overlap the root stack's dashboard and simulator
host ports; stop those services first or change the JSON device ports.

### Simulation behavior

- Each device starts with its relay off and zero power/energy.
- While on, a five-second ticker simulates approximately 50–70 W and accumulates
  energy in Wh. Allow one tick before expecting nonzero power.
- Switching off sets power to zero and preserves accumulated energy.
- State is in memory and resets on simulator restart.
- Simulation runs independently of MQTT availability; HTTP readings keep evolving
  even if the initial broker connection fails.

### Protocol reference

- Gen1: `/shelly`, `/status`, `/settings`, `/relay/0`.
  The core coding test uses this API. `/status` reports `relays[0].ison`,
  `meters[0].power` (W), and `meters[0].total` (Wh in this simulator).
- Gen2 HTTP: `/rpc/Shelly.GetDeviceInfo`, `/rpc/Shelly.GetStatus`,
  `/rpc/Switch.GetStatus`, `/rpc/Switch.Set`, `/rpc/Switch.Toggle`.
- Gen2 RPC frames and WebSocket: `/rpc`.
- Native MQTT relay commands: `shellies/<device-id>/relay/0/command`, payload
  `on`, `off`, or `toggle`.
- Native MQTT readings: `shellies/<device-id>/relay/0`, plus `/power` and `/energy`.
- MQTT RPC requests: `<device-id>/rpc`; replies go to `<src>/rpc`.
- Gen2 MQTT status notifications: `<device-id>/events/rpc`.

These native MQTT topics are separate from the exercise's `agent/<id>/status`
and `agent/<id>/command` topics. In the core exercise, the agent reads HTTP and
publishes its own telemetry. The optional MQTT extension bridges native topics.
Only native relay state is retained; native power/energy are not retained.
Native readings are published initially, after native commands, and on simulation
ticks while on. Off devices do not send periodic readings.

## Other commands

- `make fmt`, `make fmt-check`: format/check project Go files, excluding vendor.
- `make lint`: formatting, vet, and an externally installed `golangci-lint`.
- `make docker-build`: build the simulator image.
- `make docker-run`: run the simulator with its broker using root Compose.
- `make compose-up`, `make compose-down`: forward to root stack commands.
- `make clean`: remove local build output.

Dependencies are vendored. After changing dependencies, run `go mod tidy` and
`go mod vendor`. The Docker build compiles with `-mod=vendor`.
