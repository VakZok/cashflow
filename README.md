# Money Manager

Personal money tracker. See `GLOSSARY.md` for the vocabulary and `docs/adr/` for decisions.

## Run

    go run ./cmd/moneymanager -addr 127.0.0.1:8080 -db money_manager.db

The database is created and migrated on startup.

## Test

    go vet ./... && go test ./...

Tests are HTTP-level only (`internal/web`): the real app on a fresh temporary SQLite
database, driven through `httptest`. Add new tests with the helpers in
`internal/web/harness_test.go`.

## Layout

- `cmd/moneymanager` – entry point
- `internal/store` – SQLite access and versioned migrations (`migrations/NNNN_name.sql`, applied in order at startup, version kept in `PRAGMA user_version`)
- `internal/web` – routes, handlers, `html/template` templates (`templates/`)

## Toolchain and CI

The Go version is pinned in `go.mod` (see ADR 0005). CI runs style (golangci-lint), tests (-race), security (govulncheck) and a Pi build as separate jobs. Build for the Raspberry Pi 4 (Pi OS 64-bit):

    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o moneymanager ./cmd/moneymanager
