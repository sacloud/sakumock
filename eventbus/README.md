# sakumock/eventbus

An EventBus compatible mock server for local development and testing. It implements the control-plane of the [SAKURA Cloud EventBus API](https://github.com/sacloud/sacloud-sdk-go/tree/main/api/eventbus) — process configurations, schedules, and triggers — with in-memory storage, so applications and Terraform can exercise EventBus CRUD in tests.

It also has a **data plane** that *fires* those resources: schedules fire on their `Crontab`/recurring interval, and triggers fire on events injected via `POST /_sakumock/events`. Each firing resolves the referenced process configuration and is recorded as a delivery (inspectable via `GET /_sakumock/deliveries`) with the resource's `Status` updated. When [service link](../README.md#service-link) is enabled (`sakumock all --enable-service-link`), fired jobs are forwarded to their destination service over HTTP using the official SDK client. See [Data plane](#data-plane-firing) and [Service link](#service-link) below.

## Install

```bash
go install github.com/sacloud/sakumock/eventbus/cmd/sakumock-eventbus@latest
```

Or use the unified [`sakumock`](../README.md#install) binary: `sakumock eventbus` accepts the same flags as `sakumock-eventbus`.

## Usage

```bash
sakumock-eventbus
```

### Options

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--addr` | `EVENTBUS_LOCALSERVER_ADDR` | `127.0.0.1:18085` | Listen address |
| `--latency` | `EVENTBUS_LATENCY` | `0` | Artificial latency added to every response (e.g. `500ms`, `2s`) |
| `--rate-limit` | `EVENTBUS_RATE_LIMIT` | `0` | HTTP rate limit on the API endpoints (events per `--rate-limit-window`, `0` disables). Excess requests get `429 Too Many Requests` with a `Retry-After` header |
| `--rate-limit-window` | `EVENTBUS_RATE_LIMIT_WINDOW` | `1s` | Window for `--rate-limit` (e.g. `1s`, `1m`) |
| `--enable-data-plane` | `EVENTBUS_ENABLE_DATA_PLANE` | `false` | Run the autonomous scheduler that fires schedules on the wall clock. The `/_sakumock` firing endpoints work regardless of this flag |
| `--debug` | `EVENTBUS_DEBUG` | `false` | Enable debug mode |
| `--tls-cert` | `EVENTBUS_TLS_CERT` | (none) | TLS certificate file; with `--tls-key`, the server serves HTTPS instead of plain HTTP |
| `--tls-key` | `EVENTBUS_TLS_KEY` | (none) | TLS key file (see `--tls-cert`) |

## Use with sacloud-sdk-go

The [sacloud-sdk-go](https://github.com/sacloud/sacloud-sdk-go) `api/eventbus` client reads the `SAKURA_ENDPOINTS_EVENTBUS` override:

```bash
export SAKURA_ENDPOINTS_EVENTBUS=http://localhost:18085/
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

Keep the trailing slash on the endpoint URL. The SDK matches the list-API path with `url.JoinPath`, which drops the leading slash when the endpoint URL has an empty path; without the slash the SDK never sends the `Provider.Class` filter and `List` returns process configurations, schedules, and triggers mixed together. (`sakumock env` and the startup log emit the URL with the slash already in place.)

## Library usage

```go
import "github.com/sacloud/sakumock/eventbus"

// As http.Handler (for custom servers)
handler, _ := eventbus.NewHandler(eventbus.Config{})
defer handler.Close()

// As test server (for integration tests)
srv := eventbus.NewTestServer(eventbus.Config{})
defer srv.Close()
fmt.Println(srv.TestURL()) // http://127.0.0.1:<random-port>

// Assert the write-only secret an application set on a process configuration
if secret, ok := srv.Secret(id); ok {
    fmt.Println(string(secret))
}

// Drive and inspect firing without a live destination service
http.Post(srv.TestURL()+"/_sakumock/events", "application/json",
    strings.NewReader(`{"Source":"//monitoringsuite...","Attributes":{"status":"critical"}}`))
for _, d := range srv.Deliveries() {
    fmt.Println(d.SourceID, d.Destination, d.Parameters)
}

// InspectionClient — drive and inspect firing over HTTP (e.g. against a
// running sakumock process or container, not just in-process test servers)
ic := eventbus.NewInspectionClient("http://localhost:18085")
ds, _ := ic.InjectEvent(ctx, eventbus.Event{Source: "//monitoringsuite..."})
ds, _ = ic.Tick(ctx, time.Now())   // force scheduler evaluation
ds, _ = ic.Deliveries(ctx)         // list all recorded firings
_ = ic.ClearDeliveries(ctx)        // reset
```

## API endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/commonserviceitem` | Create a process configuration, schedule, or trigger |
| GET | `/commonserviceitem` | List items, filterable with `?{"Filter":{"Provider.Class":"..."}}` |
| GET | `/commonserviceitem/{id}` | Get an item |
| PUT | `/commonserviceitem/{id}` | Update an item |
| DELETE | `/commonserviceitem/{id}` | Delete an item |
| PUT | `/commonserviceitem/{id}/eventbus/processconfiguration/set-secret` | Set the secret of a process configuration |
| POST | `/_sakumock/events` | Inject an event and fire matching triggers (mock-only) |
| POST | `/_sakumock/tick` | Evaluate schedules and fire those due, optional `?at=<time>` (mock-only) |
| GET | `/_sakumock/deliveries` | List recorded firings (mock-only) |
| DELETE | `/_sakumock/deliveries` | Clear recorded firings (mock-only) |

The `/_sakumock/` endpoints do not exist in the real API; they drive and observe the data plane (see [Data plane](#data-plane-firing)).

Behavior notes:

- `Provider.Class` must be `eventbusprocessconfiguration`, `eventbusschedule`, or `eventbustrigger`; per-class required settings are validated (`Destination`/`Parameters`, `ProcessConfigurationID`/`StartsAt`, `Source`/`ProcessConfigurationID`).
- A schedule must specify its type — exactly one of `Crontab` or a recurring interval (`RecurringStep` with `RecurringUnit`). The control panel presents these as a mutually exclusive choice, but the OpenAPI marks both optional and cannot express it, so the mock enforces it: a schedule with neither, or with both, fails with `400`.
- Schedules and triggers must reference an existing process configuration; a create or update pointing at a missing ID fails with `400`.
- `StartsAt` is accepted as an integer (epoch milliseconds) and returned as a string, matching the real API.
- Secrets are write-only: `set-secret` stores them, but no API response ever includes them. Test code can read them back with `Server.Secret(id)`.
- `Availability` is always `available`. A schedule's or trigger's `Status` is unset until it fires, then reflects the latest firing (see [Data plane](#data-plane-firing)).

### Crontab schedules

A schedule's `Crontab` is validated against the notation published in the [EventBus manual](https://manual.sakura.ad.jp/cloud/appliance/eventbus/control_panel.html) (指定可能なCrontab形式の記法): five space-separated numeric fields (minute, hour, day, month, day-of-week 0–6 with 0=Sunday) using only the symbols `*` `,` `-` `/`. Per the documented restrictions, day-of-week `7`, month/day names (`JAN`, `MON`), aliases (`@yearly`), and extended operators (`L` `W` `#` `?` `H`) are rejected with `400`, so an expression the real API would refuse fails in the mock too. The parser is exposed as `eventbus.ParseCrontab` with `Matches`/`Next` for evaluation.

Two points the published spec does not define; the mock's choices are documented here so you know what you are relying on:

- **Timezone**: crontab expressions are evaluated in **JST (UTC+9, no DST)**. The real service's evaluation timezone is not documented; JST is an assumption.
- **Day matching**: when both the day-of-month and day-of-week fields are restricted (neither is `*`), the schedule fires when *either* matches (the classic Vixie cron rule).

Note: the manual's example table describes `0 0 1 */2 *` as firing in even months, but standard cron semantics for `*/2` in the month field (stepping from the range start, 1) give the odd months 1, 3, 5, 7, 9, 11. The mock follows standard cron semantics.

## Data plane (firing)

EventBus has two kinds of event source, and both resolve to the same action — fire the referenced process configuration:

- **Schedules are time-driven** and self-contained: the mock evaluates a `Crontab` expression or a `RecurringStep`/`RecurringUnit` interval (measured from `StartsAt`) against the clock. With `--enable-data-plane`, a scheduler loop ticks once a minute and fires schedules as their boundaries pass. Regardless of the flag, `POST /_sakumock/tick?at=<time>` forces an evaluation at a chosen time, so tests fire schedules deterministically without waiting on the wall clock. `at` is an RFC3339 timestamp (e.g. `2024-01-01T09:00:00+09:00`) or, for convenience, bare epoch seconds (second resolution is enough since schedules fire at minute granularity); it defaults to now. A schedule fires once for every boundary in the half-open window `(previous tick, at]`.
- **Triggers are event-driven**: they react to an external event (a monitoringsuite alert, an eventlog entry) the mock cannot observe. Instead, `POST /_sakumock/events` injects an event and the mock matches it against every trigger:

  ```json
  { "Source": "...", "Type": "...", "Attributes": { "status": "critical" }, "Data": { } }
  ```

  A trigger fires when `Source` matches exactly, `Type` is among the trigger's `Types` (when set), and every `Condition` (`eq`/`in` over `Attributes`) holds. `Data` is the passthrough payload (the API reserves the `data` key from conditions) and is not used for matching.

Each firing is recorded as a **delivery** — the resolved `Destination` and `Parameters` of the process configuration — and the schedule's or trigger's `Status` is updated. Recorded deliveries are returned by `GET /_sakumock/deliveries` and, in library use, by `Server.Deliveries()`, so a test can assert what fired without a live destination. A firing whose process configuration is missing is recorded with an `Error` and a failed `Status`.

## Service Link

When running under `sakumock all --enable-service-link`, fired jobs are forwarded to their destination service over HTTP using the official SDK client. The forwarder applies a 5-second timeout per delivery.

| Destination | Parameters | SDK call |
|---|---|---|
| `simplemq` | `{"queue_name": "...", "content": "..."}` | `simplemqsdk.NewMessageOp.Send` |
| `simplenotification` | `{"group_id": "...", "message": "..."}` | `simplenotificationsdk.NewGroupOp.SendMessage` |

Without service link (the default, or when running standalone), firings are recorded but not forwarded — `GET /_sakumock/deliveries` and `Server.Deliveries()` still show them.

Service link requires the data plane to be enabled as well:

```bash
sakumock all --enable-service-link --eventbus-enable-data-plane
```

The `autoscale` destination has no mock and is silently ignored.

### Not yet implemented

- **Real event producers**: trigger events arrive only through `POST /_sakumock/events`. Wiring a producer such as a monitoringsuite alert-fire endpoint that emits events here is intentionally a separate service change. The `//eventbus.sakura.ad.jp/eventlog` source has no producer in sakumock and is out of scope.
