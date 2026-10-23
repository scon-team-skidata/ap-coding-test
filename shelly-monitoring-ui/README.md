# Shelly monitoring UI

Working Go dashboard for the [agent coding test](../shelly-agent/CODING_TEST.md).
HTML, CSS, and JavaScript are embedded into the binary; no frontend build tools
or browser MQTT connection are needed.

## Run

From the repository root:

```sh
make up
```

Without Make, run `docker compose up --build -d` from the root. From
`shelly-monitoring-ui/`, the equivalent is:

```sh
docker compose -f ../docker-compose.yml up --build -d
```

All services belong to this shared root stack; there is no UI-specific Compose
file. Use `docker compose -f ../docker-compose.yml logs -f ui` from this directory
to follow UI logs, or `make -C .. down` to stop the shared stack.

Open http://localhost:8080. To run the UI locally against the root stack's broker:

```sh
cd shelly-monitoring-ui
make run
```

The local Go server listens on http://localhost:8090 by default. Compose maps
host port 8080 to this internal port.

Project commands: `make build`, `make test`, `make vet`.
Run these commands from `shelly-monitoring-ui/`. `make compose-up` and
`make compose-down` forward to the shared root stack commands.
Dependencies are vendored for Docker builds without module downloads. After
updating dependencies, run `go mod tidy` and `go mod vendor`.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `HTTP_ADDR` | `:8090` | HTTP listen address; use `127.0.0.1:8090` for host-only local runs |
| `MQTT_BROKER` | `tcp://localhost:1883` | Local anonymous MQTT broker; Compose uses `tcp://mosquitto:1883` |
| `STALE_AFTER` | `20s` | Observation age after which a reading is marked stale |

This is a local development dashboard with no login. The root Compose stack
publishes its port on loopback. Changing root `.env` ports only affects Compose
host mappings; adjust `MQTT_BROKER` separately when running Go locally.

## Agent integration

The core agent exercise reads simulator Gen1 HTTP and publishes normalized MQTT
telemetry. Optional Gen2 RPC or native MQTT simulator integration uses the same
dashboard contract. The optional simulator auth/TLS mode does not change UI
settings: the UI talks only to the local MQTT broker, not to the simulator API.

Expected status topic: `agent/<device-id>/status`, for example:

```json
{
  "device_id": "shellyplug-s-SIM01",
  "output": true,
  "power_w": 55.2,
  "energy_wh": 1.25,
  "observed_at": "2026-01-01T12:00:00Z"
}
```

`observed_at` is the real observation time in RFC 3339 format. The example's old
timestamp will display as stale; use the current time when testing.

## Behavior and contract

- Subscribes at QoS 1 to `agent/+/status`, discovering devices automatically.
- Displays relay state, power in watts, total energy in watt-hours, and
  observation age. MQTT disconnection and browser stream disconnection are shown.
- Accepts the telemetry schema from the agent exercise. Missing required fields,
  topic/payload ID mismatches, negative measurements, and malformed payloads are
  ignored and logged. IDs support letters, digits, `_`, and `-` (1–128 characters).
- Keeps the newest observation per device in memory. Older out-of-order readings
  do not replace it. State is not persisted; retained telemetry can repopulate it
  after restart if the agent chooses to publish retained status.
- On/off buttons publish `{"on":true}` / `{"on":false}` to
  `agent/<device-id>/command` at **QoS 1, not retained**. Duplicate delivery is
  possible; setting a desired state is idempotent.
- A command is marked sent after MQTT acknowledgement. Only subsequent telemetry
  confirms actual device state. A timeout can mean delivery is uncertain; commands
  are not explicitly retried by the UI or persisted for offline delivery.
- Reconnects to MQTT after startup failures and connection loss, and resubscribes.
- SSE streams snapshots to browsers. Slow clients receive the latest snapshot
  rather than an unbounded queue. Browser reconnection is automatic.
- HTTP shutdown and MQTT disconnect run when SIGINT or SIGTERM is received.

HTTP endpoints:

| Endpoint | Purpose |
| --- | --- |
| `GET /` | Dashboard |
| `GET /api/state` | Current MQTT status and device snapshot |
| `GET /api/events` | Live snapshots via server-sent events |
| `POST /api/devices/{id}/command` | JSON body `{"on":true}` or `{"on":false}`; returns 202 once sent |

The UI never calls the simulator directly. Before the candidate implements the
agent, use the synthetic MQTT example in the [root README](../README.md) to
exercise the dashboard.

## Troubleshooting

- **Waiting for telemetry:** the provided agent on `main` is a skeleton. Implement
  it or publish synthetic status using the root README example. Native
  `shellies/...` simulator messages do not create dashboard devices.
- **Stale telemetry:** no sufficiently recent observation has arrived. Old
  retained readings remain visible; the UI does not invent fresh timestamps.
- **Command sent but unchanged:** broker acknowledgement is not device execution.
  Check agent logs and wait for a fresh status reading.
- **Zero measurements:** the simulator starts off; when turned on, allow a
  five-second simulation tick and the agent's next observation. Energy resets
  on simulator restart.
