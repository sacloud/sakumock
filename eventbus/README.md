# sakumock/eventbus

An EventBus compatible mock server for local development and testing. It implements the control-plane of the [SAKURA Cloud EventBus API](https://github.com/sacloud/sacloud-sdk-go/tree/main/api/eventbus) — process configurations, schedules, and triggers — with in-memory storage. Scheduled or triggered job execution is out of scope: this mock stores the resources so applications and Terraform can exercise EventBus CRUD in tests, but never dispatches jobs to their destinations.

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
| `--debug` | `EVENTBUS_DEBUG` | `false` | Enable debug mode |

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

Behavior notes:

- `Provider.Class` must be `eventbusprocessconfiguration`, `eventbusschedule`, or `eventbustrigger`; per-class required settings are validated (`Destination`/`Parameters`, `ProcessConfigurationID`/`StartsAt`, `Source`/`ProcessConfigurationID`).
- Schedules and triggers must reference an existing process configuration; a create or update pointing at a missing ID fails with `400`.
- `StartsAt` is accepted as an integer (epoch milliseconds) and returned as a string, matching the real API.
- Secrets are write-only: `set-secret` stores them, but no API response ever includes them. Test code can read them back with `Server.Secret(id)`.
- Job execution is not simulated; `Availability` is always `available` and a resource's `Status` is never populated.

### Crontab schedules

A schedule's `Crontab` is validated against the notation published in the [EventBus manual](https://manual.sakura.ad.jp/cloud/appliance/eventbus/control_panel.html) (指定可能なCrontab形式の記法): five space-separated numeric fields (minute, hour, day, month, day-of-week 0–6 with 0=Sunday) using only the symbols `*` `,` `-` `/`. Per the documented restrictions, day-of-week `7`, month/day names (`JAN`, `MON`), aliases (`@yearly`), and extended operators (`L` `W` `#` `?` `H`) are rejected with `400`, so an expression the real API would refuse fails in the mock too. The parser is exposed as `eventbus.ParseCrontab` with `Matches`/`Next` for evaluation.

Two points the published spec does not define; the mock's choices are documented here so you know what you are relying on:

- **Timezone**: crontab expressions are evaluated in **JST (UTC+9, no DST)**. The real service's evaluation timezone is not documented; JST is an assumption.
- **Day matching**: when both the day-of-month and day-of-week fields are restricted (neither is `*`), the schedule fires when *either* matches (the classic Vixie cron rule).

Note: the manual's example table describes `0 0 1 */2 *` as firing in even months, but standard cron semantics for `*/2` in the month field (stepping from the range start, 1) give the odd months 1, 3, 5, 7, 9, 11. The mock follows standard cron semantics.

## TODO: cross-service delivery (data plane)

Job execution is not simulated yet. Since sakumock also mocks the delivery targets, the data plane could be implemented within `sakumock all`:

- **Schedule firing → simplemq / simplenotification**: a scheduler loop evaluates `StartsAt` + `RecurringStep`/`RecurringUnit` and `Crontab` (the parser and `Next` already exist, see above) and delivers the process configuration's `Parameters` to the simplemq / simplenotification mock over HTTP. This makes the realistic dev loop work locally: EventBus schedule periodically enqueues → the application consumes the queue. The `autoscale` destination has no mock and would only be logged.
- **Trigger firing**: monitoringsuite does not evaluate alerts, so trigger events need a simulation endpoint (e.g. a `/_sakumock/` route on monitoringsuite that emits an alert event to eventbus), then eventbus matches `Source`/`Types`/`Conditions` and dispatches the same way. The `//eventbus.sakura.ad.jp/eventlog` source has no event source in sakumock and is out of scope.
- Wiring: services must not import each other, so delivery goes over HTTP. The unified binary would inject the peer services' endpoints via `core.ServerOptions`; standalone use and tests keep today's CRUD-only behavior. Job results would populate the currently-unused `Status` field, and delivery could require the secret to have been set (as the real service does).
