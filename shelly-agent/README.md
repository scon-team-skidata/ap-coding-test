# shelly-agent

Go agent skeleton for the Shelly coding test. Replace the TODO templates with a
working agent; see [CODING_TEST.md](CODING_TEST.md).

The core exercise integrates with the simulator over Gen1 HTTP. Optional
extensions include Gen2 HTTP RPC, Gen2 WebSocket RPC, native MQTT integration,
and API authentication/TLS; see [OPTIONAL_EXTENSIONS.md](OPTIONAL_EXTENSIONS.md).
The agent-to-dashboard MQTT contract remains the same for every simulator transport.

## Layout

- `main.go` — signal-aware entrypoint.
- `agent.go` — placeholder `run(ctx)` and optional `device`/`publisher` testing interfaces.
- `config.go` — configuration struct and parsing/validation TODO.
- `shelly.go` — suggested Gen1 client type and unimplemented method signatures.
- `mqtt.go` — suggested MQTT client type and unimplemented lifecycle/messaging methods.
- `command.go` — working strict command parser (`parseCommand`), ready to call.
- `command_test.go` — supplied command validation tests, including `false`, missing
  fields, duplicate/unknown fields, and the 1024-byte payload limit.
- `telemetry.go` — prescribed dashboard-facing JSON payload type.
- `CODING_TEST.md` — implementation tasks, questions, and acceptance criteria.
- `OPTIONAL_EXTENSIONS.md` — independent optional tasks and protocol specifications.
- `ANSWERS.md` — template for your discussion answers and submission notes.
- `.env.example` — host-side development configuration for the exercise.
- `Makefile` — build, test (incl. race detector), vet, fmt, run, and Compose commands.
- `.dockerignore` — excludes local configuration and build artifacts from Docker builds.
- `go.mod`, `go.sum`, `vendor/` — Go 1.24 module with Paho MQTT dependencies supplied.
- `Dockerfile` — vendored multi-stage build and non-root `scratch` runtime image.
- The root [`docker-compose.yml`](../docker-compose.yml) starts:
  - `mosquitto`: MQTT broker (plain, anonymous, `tcp://mosquitto:1883`).
  - `simulator`: the [`shelly-simulator`](../shelly-simulator) from the sibling directory,
    built from its own Dockerfile, using **HTTP without authentication or TLS**.
  - `agent`: this application, built from the local `Dockerfile`.
  - `ui`: the working monitoring dashboard at `http://localhost:8080`.

The file structure and client signatures are suggestions; you may change them.
Unimplemented helpers return explicit TODO errors rather than fabricated readings.
`run(ctx)` does not call those helpers yet, so the starting application builds and
waits for shutdown. Command parsing and its unit tests already work. Configuration
parsing, HTTP requests/decoding, MQTT messaging, calling the parser before executing
commands, retries, concurrency, and additional tests are candidate work.

The optional interfaces in `agent.go` let you test orchestration with small fake
devices and publishers. They do not prescribe a worker or transport architecture.
The concrete clients still own discovery and network lifecycle operations.

## Running

From this project directory (`shelly-agent/`), start the shared Docker environment:

```sh
make -C .. up
```

Direct Compose equivalent, also from `shelly-agent/`:

```sh
docker compose -f ../docker-compose.yml up --build -d
```

Use the root stack for every project; there is no `shelly-agent/docker-compose.yml`.
Optional secure mode (auth/TLS) is described in the [root README](../README.md#optional-authentication-and-tls)
and [OPTIONAL_EXTENSIONS.md](OPTIONAL_EXTENSIONS.md#optional-authentication-and-tls-mode).

Services:

| Service   | Address                        | Notes                              |
|-----------|---------------------------------|-------------------------------------|
| mosquitto | `tcp://localhost:1883`          | anonymous, unencrypted              |
| simulator | `http://localhost:8081`         | plain HTTP, no authentication      |
| ui        | `http://localhost:8080`         | live telemetry and on/off controls |
| agent     | —                                | background worker, no exposed ports |

Example call against the simulator:

```sh
curl -s http://localhost:8081/status
```

The core endpoints are `GET /shelly` (device ID), `GET /status` (relay state,
power, and energy), and `GET /relay/0?turn=on|off` (control). The simulator reports
power in W and total energy in Wh. It starts off and updates measurements every
five seconds while on.

The UI remains empty until an agent publishes `agent/<device-id>/status`.
Buttons publish `{"on":true}` / `{"on":false}` to
`agent/<device-id>/command`; the agent must apply these commands to the simulator
and publish fresh observed state. See the [root README](../README.md) for the
synthetic telemetry smoke test.

## Local Go development

Run these commands from `shelly-agent/`. Start the shared environment first, then
stop its placeholder agent to avoid two instances after implementing your code:

```sh
docker compose -f ../docker-compose.yml stop agent
```

Leave the simulator, broker, and UI running. After local development, stop the
local Go process and run `make -C .. up` to restore the containerized agent.

```sh
cp .env.example .env
set -a
. ./.env
set +a
make build
make run
```

The skeleton logs a startup message and waits for SIGINT or SIGTERM. Environment
variable parsing and application behavior are part of the coding test; `.env`
is not loaded automatically by the Go application.

The example environment uses host addresses. The Compose agent uses
`http://simulator:8080` and `tcp://mosquitto:1883` instead. Adjust the project's
`.env` if you override host ports in the root `.env`.
The root Compose file already passes the exercise's configuration variables to
the agent. Implement their parsing as part of the test. The root `.env` controls
stack settings; this project's `.env` is for host-side Go development.

Development commands:

```sh
make test         # go test ./...
make test-race    # go test -race ./...
make vet          # go vet ./...
make fmt          # gofmt the module (excludes vendor/)
make check        # test and vet
make compose-up   # start the shared root stack in the background
make compose-down # stop the shared root stack
```

Dependencies are checked in under `vendor/` so Docker builds need no module
downloads. Paho is supplied and imported by the MQTT client template. After
changing dependencies, run `go mod tidy` and `go mod vendor`, and commit `go.mod`,
`go.sum`, and `vendor/` together.

## Manual verification checklist

Run these commands from the **repository root**, after implementing the core
agent. They assume the default HTTP/no-auth stack and default host ports; adjust
the curl URL if you override `SIMULATOR_HTTP_PORT`. Use a separate terminal for
the long-running subscriber. MQTT clients run inside the broker container.

### 1. Start and observe

```sh
make up
docker compose logs --tail 50 agent
docker compose exec mosquitto mosquitto_sub -v -t 'agent/+/status'
```

Expect periodic `agent/shellyplug-s-SIM01/status` messages with real readings,
UTC `observed_at`, and energy in Wh. Open http://localhost:8080 and check the
same device appears. The unimplemented skeleton will not publish anything.

### 2. Turn on and off

```sh
docker compose exec mosquitto mosquitto_pub \
  -t agent/shellyplug-s-SIM01/command -m '{"on":true}'
curl -fsS http://localhost:8081/status
```

Wait for observed telemetry to report `output:true`; the HTTP relay should be
on. Allow a five-second simulator tick plus polling time for nonzero power.
Then verify off:

```sh
docker compose exec mosquitto mosquitto_pub \
  -t agent/shellyplug-s-SIM01/command -m '{"on":false}'
curl -fsS http://localhost:8081/status
```

Wait for `output:false`, zero power, and preserved energy. MQTT acceptance does
not mean execution has finished, so an immediate curl may precede the switch.
Repeat using the dashboard's buttons; observed state should change only after
the agent reads the device.

### 3. Reject invalid commands

With the relay off, send commands that must not turn it on:

```sh
docker compose exec mosquitto mosquitto_pub \
  -t agent/shellyplug-s-SIM01/command -m '{}'
docker compose exec mosquitto mosquitto_pub \
  -t agent/shellyplug-s-SIM01/command -m '{"on":true,"extra":1}'
docker compose logs --tail 50 agent
curl -fsS http://localhost:8081/status
```

Expect rejection reasons in logs, the relay still off, and continued telemetry.
The supplied parser tests cover other malformed shapes and both boolean values.

To check rejection of old retained intent, leave the relay off, stop the agent,
and publish a retained on command. Start the agent and confirm the relay remains
off and the rejection is logged. Clear the retained message afterward:

```sh
docker compose stop agent
docker compose exec mosquitto mosquitto_pub -r \
  -t agent/shellyplug-s-SIM01/command -m '{"on":true}'
docker compose start agent
docker compose logs --tail 50 agent
curl -fsS http://localhost:8081/status
docker compose exec mosquitto mosquitto_pub -r -n \
  -t agent/shellyplug-s-SIM01/command
```

Stopping the agent first ensures this checks retained replay on subscription,
rather than delivery of the initial publication to an existing subscriber.

### 4. Recover from outages

```sh
docker compose stop mosquitto
docker compose logs --tail 50 agent
docker compose start mosquitto
```

Expect connection errors followed by recovery and command resubscription.
Restart the subscriber if it exited, then repeat the on/off check.

```sh
docker compose stop simulator
docker compose logs --tail 50 agent
docker compose start simulator
```

Expect failed reads without fresh successful telemetry during the outage and
new observations after recovery. The simulator restarts off with zero energy.
Also verify startup recovery by stopping `simulator` and `mosquitto`, restarting
the already-created agent with `docker compose restart agent`, then starting
the dependencies again. The agent should stay running and eventually recover.

### 5. Shut down

```sh
docker compose stop -t 5 agent
docker compose logs --tail 50 agent
docker compose ps -a agent
make down
```

Check the agent stopped within its documented shutdown bound without requiring
SIGKILL (exit code 137), polling stopped, and network resources were released.

## Reference documentation

- [Coding-test instructions](CODING_TEST.md): core contract and acceptance criteria.
- [Optional extensions](OPTIONAL_EXTENSIONS.md): alternate transports, TLS,
  multiple devices, availability, and integration testing.
- [Go HTTP client](https://pkg.go.dev/net/http#Client) and
  [context](https://pkg.go.dev/context): request bounds and cancellation.
- [Go httptest](https://pkg.go.dev/net/http/httptest): isolated HTTP test servers.
- [Paho MQTT Go](https://pkg.go.dev/github.com/eclipse/paho.mqtt.golang): options,
  tokens, and callback behavior; consult the version pinned in `go.mod`.
- [Shelly Gen1](https://shelly-api-docs.shelly.cloud/gen1/) and
  [Gen2](https://shelly-api-docs.shelly.cloud/gen2/): real-device background.

The repository's simulator contract takes precedence over real-device examples,
particularly for energy units and authentication.
