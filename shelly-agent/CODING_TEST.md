# Coding test: Shelly agent

## Goal

Implement a small Go service that connects a simulated Shelly smart plug to an
MQTT broker. Retrieve device status over plain HTTP without authentication, publish telemetry,
and accept MQTT commands to control the plug.

Suggested timebox: **2–3 hours**. Prioritize a working, readable core solution.
Document unfinished work and tradeoffs. Optional extensions are independent
choices outside the core timebox; implementing all of them is not expected.

## Starting point

- This repository contains a signal-aware Go entrypoint and a placeholder
  `run(ctx)` in `agent.go`, suggested configuration/client templates in `config.go`,
  `shelly.go`, and `mqtt.go`, and the telemetry JSON type in `telemetry.go`.
- `command.go` provides a working `parseCommand` helper, with validation tests in
  `command_test.go`. Call it before executing commands; you do not need to
  reimplement it. Optional `device` and `publisher` interfaces in `agent.go`
  support small test doubles and may be changed or removed.
- A Dockerfile, Makefile, `.env.example`, and vendored Paho MQTT dependencies are
  supplied. The templates compile; unimplemented helpers return TODO errors and
  are not called by the placeholder. You may change the suggested structure.
- The root `../docker-compose.yml` starts the agent, a Shelly simulator,
  Mosquitto, and a working monitoring UI at `http://localhost:8080`.
- The simulator source is available in the sibling `../shelly-simulator` directory.
- The simulator API uses HTTP without authentication or TLS by default.
- Authentication and TLS are optional extensions, not required for the core test.
- MQTT is anonymous and unencrypted for this local exercise.

From `shelly-agent/`, start the environment with:

```sh
make -C .. up
```

Inside Compose, use `http://simulator:8080` and `tcp://mosquitto:1883`.
From the host, use `http://localhost:8081` and `tcp://localhost:1883`.
The initial agent waits for SIGINT or SIGTERM without doing any application work.
Replace the placeholder in `run(ctx)` and implement the template TODOs. Integrating
the supplied parser, application behavior, and tests beyond command parsing remain
your responsibility. See the [README layout](README.md#layout) for supplied files.

## Implementation tasks

### 1. Configuration

Read configuration from environment variables and document defaults and required
values. Support at least:

| Variable | Purpose | Example |
| --- | --- | --- |
| `SHELLY_URL` | Simulator HTTP base URL | `http://simulator:8080` |
| `MQTT_BROKER` | MQTT broker address | `tcp://mosquitto:1883` |
| `POLL_INTERVAL` | Status polling interval | `5s` |
| `HTTP_TIMEOUT` | Per-request timeout | `3s` |

Reject invalid configuration with a useful error. Credentials and TLS settings
are not required for the core implementation.

The root Compose file passes these variables to the agent.
Implement parsing and validation; update the service if
your solution needs additional configuration.

### 2. Simulator integration

Use the simulator's **Gen1 HTTP API**, without authentication, to:

1. Retrieve device information with `GET /shelly`.
2. Retrieve status immediately and then periodically with
   `GET /status`.
3. Control relay `0` with `GET /relay/0?turn=on` and
   `GET /relay/0?turn=off`.

Example requests from the host:

```sh
curl -s http://localhost:8081/shelly

curl -s http://localhost:8081/status

curl -s 'http://localhost:8081/relay/0?turn=on'
curl -s 'http://localhost:8081/relay/0?turn=off'
```

Device information includes `id`, for example `shellyplug-s-SIM01`. For dashboard
compatibility, accept IDs of 1–128 ASCII letters, digits, underscores (`_`), or
hyphens (`-`); reject other IDs before constructing topics. Status has
this structure (values vary):

```json
{
  "relays": [{ "ison": true }],
  "meters": [{ "power": 42.5, "total": 1.25 }],
  "uptime": 30
}
```

Map `relays[0].ison` to `output`, `meters[0].power` to `power_w`, and
`meters[0].total` to `energy_wh`. **This simulator returns total energy in Wh**;
do not apply a conversion based on a real device's Gen1 energy units.

Check HTTP status codes, handle malformed responses and missing/empty relay or
meter arrays, and close response bodies. Relay commands return JSON including
`ison`; fetch `/status` afterward for the complete telemetry reading. Use explicit
on/off commands rather than toggle so duplicate delivery preserves desired state.

References: [Go HTTP client](https://pkg.go.dev/net/http#Client) (timeouts, client
reuse, response-body cleanup), [Go context](https://pkg.go.dev/context)
(cancellation), and [Shelly Gen1 API](https://shelly-api-docs.shelly.cloud/gen1/)
(background). **The simulator contract in this repository takes precedence over
real-device documentation**, particularly for energy units and authentication.

### 3. Publish telemetry

Publish each successful status reading as JSON to:

```text
agent/<device-id>/status
```

Use this payload contract:

```json
{
  "device_id": "shellyplug-s-SIM01",
  "output": true,
  "power_w": 42.5,
  "energy_wh": 1.25,
  "observed_at": "2026-01-01T12:00:00Z"
}
```

`observed_at` is the observation time in UTC, formatted as RFC 3339. Choose and
document MQTT QoS and retained-message settings. Do not publish a failed reading
as if it were a fresh successful observation.

Reference: [Paho MQTT Go API](https://pkg.go.dev/github.com/eclipse/paho.mqtt.golang)
for client options, asynchronous tokens, and callbacks. The vendored version is
pinned in `go.mod`; ensure examples match that version.

### 4. Accept switch commands

Subscribe to:

```text
agent/<device-id>/command
```

Accept only this command shape, with a required boolean `on` field:

```json
{"on": true}
```

Map `true` to `GET /relay/0?turn=on` and `false` to
`GET /relay/0?turn=off`. Both `true` and `false`
must work. Reject malformed JSON, missing or non-boolean `on` values, and
unsupported fields without changing the switch. Log the reason and keep running.

The supplied `parseCommand` also rejects duplicate keys, trailing data, and
payloads larger than 1024 bytes. Preserve those validation guarantees when
integrating it. Reject MQTT messages marked as retained so old intent is not
executed when subscribing or reconnecting. Live commands should be published
without retention.

After a successful command, fetch and publish the current status. Serialize
switch commands so concurrent deliveries have predictable processing order.
Use the agent-owned topics above rather than the simulator's native MQTT topics.

### 5. Reliability and lifecycle

- Apply timeouts to network operations.
- Keep running after temporary simulator failures and retry with a bounded delay.
  Recover if the simulator is unavailable during startup.
- Recover from MQTT connection loss and restore command subscriptions after
  reconnecting. Handle the broker being unavailable during startup.
- Include enough context in logs to identify failed operations.
- On SIGINT or SIGTERM, stop polling, cancel in-flight work, and disconnect from
  MQTT within a bounded shutdown period. Compose allows the agent 10 seconds
  (`stop_grace_period`) before SIGKILL; document your own bound.

An in-memory implementation is sufficient; persistence is not required. Document
what happens to commands and telemetry during disconnection.

### 6. Tests and documentation

The supplied command-parser tests already cover valid on/off commands, missing
or non-boolean `on`, malformed JSON, unknown/duplicate fields, and size limits.
Keep them passing. Add focused automated tests covering:

- HTTP requests and decoding a successful status response.
- HTTP failures, timeouts, malformed responses, and missing/empty status arrays.
- Command execution: `{"on":false}` reaches relay-off, invalid commands never
  reach the device, and successful commands trigger a fresh status read.

Tests should run without a live simulator or broker. Use test doubles or Go's
[`httptest`](https://pkg.go.dev/net/http/httptest) facilities where appropriate.
The optional interfaces in `agent.go` can help test command execution and failed
readings with small fakes. You may use another testing structure.

Update `README.md` with configuration, build/run/test commands, MQTT publish and
subscribe examples, and your design decisions. The application must still build
inside Docker; update the Dockerfile if your dependencies require it.

## Acceptance checklist

- [ ] `go build ./...`, `go test ./...`, `go test -race ./...`, and `go vet ./...` succeed.
- [ ] `make -C .. up` starts the working agent and its dependencies.
- [ ] The agent uses the Gen1 HTTP API without authentication for simulator requests.
- [ ] Periodic telemetry appears on `agent/shellyplug-s-SIM01/status`.
- [ ] MQTT commands can turn the switch both on and off.
- [ ] The root stack's dashboard displays telemetry and its on/off buttons control
  the simulator through the agent.
- [ ] Invalid commands do not change the switch or stop the agent.
- [ ] Retained commands are rejected, including after reconnecting.
- [ ] Restarting the broker or simulator allows the agent to recover.
- [ ] Stopping the stack shuts the agent down cleanly.

See the [manual verification checklist](README.md#manual-verification-checklist)
for commands and expected observations. Passing the supplied parser tests alone
does not verify that the agent performs these tasks.

## Questions

Fill in the supplied [ANSWERS.md](ANSWERS.md) template. Explain your reasoning;
there is no single required architecture.

1. How did you structure configuration, simulator access, MQTT messaging, and
   orchestration? What tradeoffs did you make within the timebox?
2. How would you extend the agent to support hundreds of devices with different
   credentials and polling intervals?
3. How would you provision trusted certificates and manage credentials outside
   this local development setup?
4. What happens to telemetry and commands when the broker or simulator is
   unavailable? How would you avoid stale state and unbounded buffering?
5. MQTT can deliver duplicate messages. Why is setting a desired switch state
   useful, and how would you handle deduplication for non-idempotent commands?
6. Which logs and metrics would you add to diagnose authentication failures,
   slow devices, and lost MQTT connections?

## Optional extensions

Only attempt these after the core requirements work. Choose extensions
independently; none are required within the core 2–3-hour timebox:

- Support multiple configured devices.
- Support Gen2 HTTP RPC as an alternative protocol.
- Support Gen2 RPC over WebSocket as an alternative protocol.
- Support native MQTT integration with the simulator instead of HTTP.
- Support optional HTTP Basic authentication and HTTPS.
- Support a custom CA certificate file for verified HTTPS.
- Publish online/offline availability using an MQTT last will.
- Add an integration test using the Compose environment.

See [OPTIONAL_EXTENSIONS.md](OPTIONAL_EXTENSIONS.md) for detailed protocol
contracts, setup commands, and extension-specific testing guidance. Keep Gen1
HTTP as the default and preserve the dashboard-facing MQTT contract.

## Submission

Submit the implementation, tests, updated README, and `ANSWERS.md`. Mention any
incomplete requirements, approximate time spent, and what you would improve with
more time. Keep the simulator unchanged so the solution can be evaluated against
the provided environment.
