# Money Manager

Personal finance manager; purposefully simple

## Run

```zsh
go run ./cmd/moneymanager -addr 127.0.0.1:47613 -db money_manager.db
```

The SQLite database is created and migrated on startup.


## Project structure

- `cmd/moneymanager` – entry point
- `internal/store` – SQLite access and versioned migrations (`migrations/NNNN_name.sql`, applied in order at startup, version kept in `PRAGMA user_version`)
- `internal/web` – routes, handlers, `html/template` templates (`templates/`)

## Toolchain and CI

The Go version is pinned in `go.mod` (see ADR 0005). CI (`.github/workflows/ci.yml`) runs on pushes to `main` and `integration/**`, on pull requests, and weekly (Mondays 06:00 UTC) so new vulnerability advisories surface without code changes. Separate jobs: style (golangci-lint), tests (-race), security (govulncheck), deps (`go mod tidy` must leave `go.mod`/`go.sum` unchanged, then `go mod verify`) and a Pi build, which runs last. If the weekly run fails, CI opens a `needs-triage` issue (or comments on the open one). Build for the Raspberry Pi 4 (Pi OS 64-bit):

    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o moneymanager ./cmd/moneymanager
