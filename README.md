# Shelly coding test

One Git repository with three independent Go modules. Run the commands in this
README from the `coding-test/` root unless stated otherwise.

| Project | Purpose |
| --- | --- |
| [`shelly-agent`](shelly-agent/) | Candidate skeleton; see [coding-test instructions](shelly-agent/CODING_TEST.md) |
| [`shelly-simulator`](shelly-simulator/) | Working smart-plug simulator: Gen1 HTTP, Gen2 RPC, native MQTT, and optional auth/TLS |
| [`shelly-monitoring-ui`](shelly-monitoring-ui/) | Working browser dashboard for agent telemetry and MQTT commands |

## Quick start

Requirements: Docker with Compose and Make. Local Go development additionally
needs Go 1.24+. Without Make, use `docker compose up --build -d` to start the stack.

```sh
cp .env.example .env # optional; defaults work out of the box
make up
make ps
```

Open **http://localhost:8080**. The UI initially shows “Waiting for telemetry”:
the provided agent is deliberately a skeleton. Implement it using
`shelly-agent/CODING_TEST.md` to enable real readings and switch control.

```text
Simulator <----- Gen1 HTTP -----> Agent <-- MQTT --> Mosquitto
                                                          ^
                                                          | MQTT
                                                     Monitoring UI
                                                          |
                                                   Browser (HTTP/SSE)
```

| Service | Host address | Container address |
| --- | --- | --- |
| Dashboard | `http://localhost:8080` | `http://ui:8090` |
| MQTT | `tcp://localhost:1883` | `tcp://mosquitto:1883` |
| Simulator HTTP | `http://localhost:8081` | `http://simulator:8080` |
| Simulator HTTPS (`make up-secure` only) | `https://localhost:8443` | `https://simulator:8443` |

Host ports can be overridden in root `.env`. Ports bind to loopback. The core
exercise uses **HTTP without authentication or TLS**. MQTT is anonymous and
unencrypted. The UI uses agent-owned topics, not native simulator telemetry.

Check the simulator directly:

```sh
curl -s http://localhost:8081/status
```

The core exercise uses `GET /shelly` for discovery, `GET /status` for readings,
and `GET /relay/0?turn=on|off` for control. Implement the agent using the
[coding-test instructions](shelly-agent/CODING_TEST.md). Gen2 HTTP RPC, WebSocket
RPC, and native MQTT simulator integration are optional alternatives; the UI contract stays the
same. Simulator energy values are in **Wh**.

The agent skeleton supplies compilable TODO templates, the telemetry payload
type, and vendored MQTT dependencies as the candidate starting point.

For the candidate workflow, see the [agent layout and supplied helpers](shelly-agent/README.md#layout),
[manual verification checklist](shelly-agent/README.md#manual-verification-checklist),
and [answers template](shelly-agent/ANSWERS.md). Detailed optional protocol and
security specifications live in [OPTIONAL_EXTENSIONS.md](shelly-agent/OPTIONAL_EXTENSIONS.md);
they are independent choices outside the core exercise timebox.

### Optional authentication and TLS

Run `make up-secure` to apply `docker-compose.secure.yml` on top of the default
stack. This enables simulator HTTPS and Basic auth (`admin` / `admin`, overridable
in root `.env`) and configures the agent's HTTPS URL and credentials. Supporting
these settings is an optional coding-test extension. The simulator generates a
self-signed certificate, so this mode explicitly opts the development agent into
skipping verification. Its plain HTTP endpoint also requires authentication.

```sh
make up-secure
curl -sk -u admin:admin https://localhost:8443/status
make up # return to HTTP without authentication/TLS
```

## MQTT contract

- Telemetry: `agent/<device-id>/status`
- Commands: `agent/<device-id>/command`, JSON `{"on":true}` or `{"on":false}`

See the agent exercise for the complete telemetry schema. The dashboard discovers
devices from status messages; it never changes observed switch state optimistically.
A command marked “sent” means MQTT accepted it, not that the plug executed it.
Simulator-native `shellies/...` topics are distinct from these agent-owned topics.

### Smoke-test the UI without an implemented agent

Publish a synthetic reading:

```sh
docker compose exec mosquitto mosquitto_pub \
  -t agent/demo-plug/status \
  -m "{\"device_id\":\"demo-plug\",\"output\":false,\"power_w\":0,\"energy_wh\":1.25,\"observed_at\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
```

In another terminal, subscribe and click the device's on/off buttons:

```sh
docker compose exec mosquitto mosquitto_sub -v -t 'agent/+/command'
```

The `demo-plug` command is observable on MQTT but has no backing device or agent.
Synthetic readings are not retained and disappear from
the dashboard when the UI restarts.

## Development commands

```sh
make up        # build and start the shared stack in the background
make up-secure # optional simulator auth/TLS extension
make down      # stop and remove the shared stack (either mode)
make logs      # follow all service logs
make ps        # show service state
make build     # build each Go project into its own bin/ directory
make test      # test each independent Go module
make test-race # test each module with the race detector
make vet       # vet each module
make check     # tests and vet
```

Each project also has its own Makefile. Go commands run inside the relevant
module; no shared Go workspace is necessary.

### Docker Compose commands

All services use the **root Compose files**; there are no per-project stacks.
Run these direct equivalents from `coding-test/`:

```sh
docker compose up --build -d       # default HTTP/no-auth stack
docker compose ps
docker compose logs -f
docker compose down
```

The optional secure mode requires both files when starting or rebuilding services:

```sh
docker compose -f docker-compose.yml -f docker-compose.secure.yml up --build -d
```

Use `docker compose up --build -d` to return to the default mode. `ps`, `logs`,
`exec`, `stop`, and `down` can use the base file in either mode because the service
names and Compose project are the same. Running `up` with only the base file
reconfigures services to the default HTTP/no-auth settings.

From an immediate project subdirectory, use the following forms:

```sh
docker compose -f ../docker-compose.yml up --build -d
docker compose -f ../docker-compose.yml -f ../docker-compose.secure.yml up --build -d
```

Compose resolves build contexts and mounts relative
to the root Compose file. For predictable interpolation when a project also has
a local `.env`, run stack commands from the root, or pass `--env-file ../.env`
from the project directory after creating root `.env`. The commands in these READMEs
assume the default Compose project name; use the same `-p` value everywhere if
you customize it.

The root Compose file is the shared environment for all projects. To run only
the simulator and broker, use `docker compose up --build simulator`. See the
[simulator README](shelly-simulator/README.md) for local and multi-device usage.

## Troubleshooting

- **No devices:** inspect `make logs`; the candidate agent must publish valid
  `agent/+/status` messages. Simulator-native MQTT messages do not appear here.
- **Stale telemetry:** the latest observation is older than the UI's
  `STALE_AFTER` setting (20 seconds by default).
- **Disconnected:** the UI retries MQTT connections and resubscribes; check
  Mosquitto logs. Stored readings remain visible and age normally.
- **Port conflict:** check for other processes using the configured ports or
  change root `.env` ports, then run `make up` again.
- **Power/energy zero:** the plug starts off. Turn it on and allow a five-second
  simulator tick plus the agent's polling interval. Energy accumulates only while
  on, remains when switched off, and resets when the simulator restarts.
- **Retained messages after broker restart:** Mosquitto runs without persistence,
  so restarting it discards retained messages, including any retained command
  published for the retained-rejection check.
- **Frequent broker connections:** `mosquitto-healthcheck` connects every 30 seconds
  to check broker health; those short-lived connections are expected.
