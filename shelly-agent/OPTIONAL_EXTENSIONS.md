# Optional extensions

Return to the [core exercise](CODING_TEST.md). These are independent choices to
attempt after the core works, outside its 2–3-hour timebox. Implementing every
extension is not expected. Document your choices and tradeoffs in [ANSWERS.md](ANSWERS.md).

Keep Gen1 HTTP as the default. All alternatives must preserve the core
`agent/<device-id>/status` and `agent/<device-id>/command` contract, including
command validation. The [simulator README](../shelly-simulator/README.md) and the
contracts below take precedence over real-device documentation, particularly
for energy units, authentication, and supported notifications.

## Multiple configured devices

Choose and document a configuration format with per-device addresses,
credentials, and polling intervals. Validate device IDs, prevent duplicate
ownership within the process, and isolate failures so an unavailable device
does not block the others. Keep queues and concurrent work bounded.

The simulator supports multiple devices; see its
[configuration example](../shelly-simulator/config.example.json) and
[configuration guide](../shelly-simulator/README.md#configuration). Additional
listeners need matching Compose port mappings when accessed from the host.

## Optional Gen2 HTTP RPC integration

Add a configurable protocol choice. The equivalent simulator endpoints are:

| Task | Endpoint |
| --- | --- |
| Device information | `GET /rpc/Shelly.GetDeviceInfo` |
| Status | `GET /rpc/Shelly.GetStatus` |
| Set relay state | `POST /rpc/Switch.Set`, JSON `{"id":0,"on":true}` or `{"id":0,"on":false}` |

Gen2 status uses `switch:0.output`, `switch:0.apower`, and
`switch:0.aenergy.total` (Wh in this simulator). Method-specific endpoints return
results directly, without a `result` envelope. Handle RPC errors containing
`code` and `message` even when HTTP status is 200. This option does not require
authentication/TLS. Add isolated tests for decoding, explicit on/off requests,
missing fields, and RPC errors.

Reference: [Shelly Gen2 API](https://shelly-api-docs.shelly.cloud/gen2/) for RPC
concepts and component methods.

## Optional WebSocket simulator integration

Connect to `/rpc` using:

| Mode | Host address | Container address |
| --- | --- | --- |
| Default | `ws://localhost:8081/rpc` | `ws://simulator:8080/rpc` |
| Optional TLS | `wss://localhost:8443/rpc` | `wss://simulator:8443/rpc` |

Send full Gen2 RPC frames with unique request IDs and an agent source identifier:

```json
{"id":1,"src":"shelly-agent","method":"Shelly.GetDeviceInfo"}
```

```json
{"id":2,"src":"shelly-agent","method":"Shelly.GetStatus"}
```

```json
{"id":3,"src":"shelly-agent","method":"Switch.Set","params":{"id":0,"on":true}}
```

For off commands, set `params.on` to `false`. Unlike method-specific HTTP
endpoints, successful replies wrap their result:

```json
{"id":3,"src":"shellyplug-s-SIM01","dst":"shelly-agent","result":{"was_on":false}}
```

Handle RPC failures in `error` (`code` and `message`). Map status fields as for
Gen2 HTTP. This simulator's WebSocket implementation is **request/response
only**: it does not push periodic status notifications. Request status
immediately, periodically, and after successful commands. Confirm observed state
from received status, not from command publication.

Correlate replies by ID, apply connection/request timeouts, bound pending
requests, and handle malformed frames and unexpected IDs. Serialize writes and
use one reader; polling and commands must not race to read the same connection.
A single in-flight request is sufficient; a concurrent dispatcher is optional.
On disconnect, fail pending requests, reconnect with bounded delay, and resume
polling. Document whether uncertain commands are retried. Close the connection
and stop reconnect attempts on shutdown.

No authentication is needed by default. If also supporting secure mode, send
Basic credentials on upgrade and verify TLS certificates by default. The
explicit development skip-verification option may be used for self-signed TLS.

Use a local WebSocket test server for decoding/errors, correlation, on/off
translation, timeouts, malformed frames, and disconnect/reconnect handling.
[Gorilla WebSocket](https://pkg.go.dev/github.com/gorilla/websocket) documents
connection deadlines and the one-reader/one-writer concurrency rules.
It is already vendored as an indirect Paho dependency; if importing it directly,
run `go mod tidy` and `go mod vendor` to update dependency metadata.

## Optional native MQTT simulator integration

Consume native simulator topics and translate commands while keeping the
dashboard-facing MQTT contract. Configure `DEVICE_ID` explicitly (for example,
`shellyplug-s-SIM01`) rather than discovering it over HTTP. Use a unique agent
MQTT client ID; the simulator already uses its device ID as its client ID.

The simulator's native Gen1 MQTT contract is:

| Topic | Payload | Meaning |
| --- | --- | --- |
| `shellies/<device-id>/relay/0` | `on` or `off` | Relay state (retained) |
| `shellies/<device-id>/relay/0/power` | Decimal number | Power in W |
| `shellies/<device-id>/relay/0/energy` | Decimal number | Accumulated energy in Wh |
| `shellies/<device-id>/relay/0/command` | `on` or `off` | Set desired relay state |

Subscribe to the three state topics, validate payloads, and aggregate them into
telemetry. Do not publish a complete reading until all fields have arrived.
Messages arrive separately without device observation timestamps: track receipt
times and document the aggregation/freshness policy. Never freshen cached
measurements merely because a timer fired. Explain retained-relay handling and
cache invalidation on reconnect.

Translate validated dashboard commands into native `on`/`off`, never retained.
Do not update observed state optimistically. Avoid wildcard subscriptions that
feed the agent's own telemetry back into itself. Restore native subscriptions
on reconnect. Test payload parsing, freshness, aggregation, reconnect cache
handling, and both command translations without a live broker.

Simulator behavior:

- Native state is published initially and after a native relay command.
- While on, power/energy updates are published every five seconds.
- While off, there are no periodic updates; an idle device may correctly appear
  stale. Do not synthesize fresh observations to conceal this.
- Only relay state is retained; power/energy are not retained. An agent starting
  after an idle simulator may need a native command to receive measurements.
- Simulator native subscriptions are established only on initial connection.
  After a broker restart, restart the simulator if needed before testing native
  commands. Improving the simulator is outside this exercise.

From the repository root:

```sh
docker compose exec mosquitto mosquitto_sub -v \
  -t 'shellies/shellyplug-s-SIM01/relay/0' \
  -t 'shellies/shellyplug-s-SIM01/relay/0/power' \
  -t 'shellies/shellyplug-s-SIM01/relay/0/energy'
```

In another terminal, request a real state publication (also useful while off):

```sh
docker compose exec mosquitto mosquitto_pub \
  -t shellies/shellyplug-s-SIM01/relay/0/command -m on
```

Gen2 MQTT RPC is an alternative: send full frames to `<device-id>/rpc` with a
unique request `id` and nonempty agent `src`; subscribe to `<src>/rpc` for replies.
Handle correlation, RPC errors, and timeouts. Status notifications use
`<device-id>/events/rpc`. Document which variant you implement; both are not required.

## Optional authentication and TLS mode

From the repository root:

```sh
make up-secure
```

This combines `docker-compose.yml` and `docker-compose.secure.yml`, enables
simulator authentication/TLS, and passes these additional agent settings:

| Variable | Purpose | Development value |
| --- | --- | --- |
| `SHELLY_URL` | HTTPS endpoint | `https://simulator:8443` |
| `SHELLY_USERNAME` | Basic auth username | `admin` |
| `SHELLY_PASSWORD` | Basic auth password | `admin` |
| `TLS_INSECURE_SKIP_VERIFY` | Explicit development opt-in | `true` |

Verify certificates by default, skipping only when explicitly configured. Send
credentials only when configured and never log passwords. Test authentication
failure and certificate validation. Basic authentication also applies to the
simulator's plain HTTP listener in secure mode.

```sh
curl -sk -u admin:admin https://localhost:8443/status
make up # restore HTTP without authentication/TLS
```

MQTT remains anonymous and unencrypted in either mode.

### Custom CA certificate file

Choose and document a setting such as `TLS_CA_FILE` for a PEM CA bundle. Preserve
certificate chain and hostname verification; a custom CA must not implicitly
enable skip verification. Explain whether the bundle extends or replaces system
trust. For Docker, document the read-only certificate mount and container path.
The scratch image needs a system CA bundle copied in if relying on system roots.
Test a trusted custom CA, an untrusted certificate, and missing/invalid CA files.

References: [Go TLS configuration](https://pkg.go.dev/crypto/tls#Config) and
[Go certificate pools](https://pkg.go.dev/crypto/x509#CertPool).

## Online/offline availability

Choose and document a topic such as `agent/<device-id>/availability`, its payload,
QoS, and retention policy. Configure an offline last will before connecting,
announce online after subscriptions are ready, and publish offline on graceful
shutdown. Test unexpected loss as well as graceful shutdown. State whether
availability describes the agent's broker connection or the physical device;
telemetry age is still necessary to detect stale observations. The supplied
dashboard consumes status, not availability, so this extension does not require
changing the UI.

## Compose integration test

Add an opt-in test that uses the provided stack to verify real telemetry,
on/off commands, outage recovery, and bounded shutdown. Keep normal `go test ./...`
independent of Docker. Use an isolated project name and configurable or ephemeral
ports, and clean up test-owned resources even on failure. Document how to run it
and which transports it covers. Keep the simulator unchanged.

Reference: [Docker Compose project names](https://docs.docker.com/compose/how-tos/project-name/)
for isolation from a developer's running stack.
