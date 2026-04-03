# sakumock

sakumock is a mock suite for SAKURA Cloud APIs.

## Services

Each service runs as an independent module under its own subdirectory with a separate `go.mod`.

| Service | Default Port | Module |
|---|---|---|
| simplemq | 18080 | `github.com/sacloud/sakumock/simplemq` |
| kms | 18081 | `github.com/sacloud/sakumock/kms` |
| secretmanager | 18082 | `github.com/sacloud/sakumock/secretmanager` |

New services should use the next available port in sequence (18083, 18084, ...).

## License

This project is published under [Apache 2.0 License](LICENSE).
