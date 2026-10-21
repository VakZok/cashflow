# Cashflow

Personal finance manager. Purposefully simple

## Features

- Track your income, expense, levy, rebate, investment, and divestment
	- Income: The net that actually hit your bank account. Income = Earnings - Levy
	- Expense: Anything you paid.
	- Levy: Taxes and social duty payments deducted from your earnings.
	- Rebate: Levy you got back (e.g. refunded taxes).
	- Investment: How much you invested where.
	- Divestment: How much you withdrew from an investment
- Add, edit, or remove existing transactions or categories
- Save repeating monthly transactions and add with one click
- Generate monthly or yearly reports

## Example: Divestments

Given 100€ investemt:

- sold with gain of 20€ -> 100€ divestment, 15€ income, 15€ levy
- sold with loss of 20€ -> 100€ divestment, 20€ expense

## Toolchain and CI

The Go version is pinned in `go.mod` (see ADR 0005). CI (`.github/workflows/ci.yml`) runs on pushes to `main` and `integration/**`, on pull requests, and weekly (Mondays 06:00 UTC) so new vulnerability advisories surface without code changes. Separate jobs: style (golangci-lint), tests (-race), security (govulncheck), deps (`go mod tidy` must leave `go.mod`/`go.sum` unchanged, then `go mod verify`) and a Pi build, which runs last. If the weekly run fails, CI opens a `needs-triage` issue (or comments on the open one). Build for the Raspberry Pi 4 (Pi OS 64-bit):

    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o moneymanager ./cmd/moneymanager

## Run (compiled binary)

```zsh
./moneymanager -addr 127.0.0.1:47613 -db money_manager.db
```

The SQLite database is created and migrated on startup.

Dummy data for trying out the Report (refuses a non-empty database):

```zsh
go run ./cmd/seed -db money_manager.db -months 3
```

## Known limitations

- Search only ignores case for ASCII letters: "a" matches "A", but "ä" does not match "Ä".