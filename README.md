# sakumock

A local mock server suite for SAKURA Cloud APIs, inspired by [LocalStack](https://github.com/localstack/localstack). Run SAKURA Cloud services locally for development and testing without connecting to the real API.

## Services

Each service runs as an independent Go module with its own binary. See the individual README for details.

| Service | Default Port | Module | Description |
|---|---|---|---|
| [simplemq](simplemq/) | 18080 | `github.com/sacloud/sakumock/simplemq` | SimpleMQ message API |
| kms | 18081 | `github.com/sacloud/sakumock/kms` | KMS encrypt/decrypt API (planned) |
| [secretmanager](secretmanager/) | 18082 | `github.com/sacloud/sakumock/secretmanager` | SecretManager API |

New services should use the next available port in sequence (18083, 18084, ...).

## Quick Start

### Install

```bash
# Install individual services
go install github.com/sacloud/sakumock/simplemq/cmd/sakumock-simplemq@latest
go install github.com/sacloud/sakumock/secretmanager/cmd/sakumock-secretmanager@latest
```

### Run

```bash
# Start mock servers
sakumock-simplemq &
sakumock-secretmanager &
```

### Connect your application

Point the SAKURA Cloud SDK to the local mock servers using `SAKURA_ENDPOINTS_*` environment variables:

```bash
# SimpleMQ
export SAKURA_ENDPOINTS_SIMPLE_MQ_MESSAGE=http://localhost:18080
# SecretManager
export SAKURA_ENDPOINTS_SECRETMANAGER=http://localhost:18082

# Dummy credentials (required by SDK, not validated by mock)
export SAKURA_ACCESS_TOKEN=dummy
export SAKURA_ACCESS_TOKEN_SECRET=dummy
```

### Use as a library in tests

Each service can also be used as a Go library to spin up in-process test servers:

```go
import "github.com/sacloud/sakumock/secretmanager"

srv := secretmanager.NewTestServer(secretmanager.Config{})
defer srv.Close()
// srv.TestURL() returns http://127.0.0.1:<random-port>
```

## License

This project is published under [Apache 2.0 License](LICENSE).
