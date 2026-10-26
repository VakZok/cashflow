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

## Deployment (Linux host)

The app runs on a Linux host on your own hardware: a Raspberry Pi or a VM on a home server, never a rented cloud VM (ADR 0003). Both use systemd, so the steps are the same.

**Never expose the app publicly.** It has no login; Tailscale membership is the only access control. Do not open or forward its port on your router or firewall, and do not use `tailscale funnel`. The app listens on `127.0.0.1` only and is reached over your tailnet via `tailscale serve`.

1. **Build** on your machine. For a Raspberry Pi 4/5 (64-bit Pi OS), the same command CI runs:

       CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o moneymanager ./cmd/moneymanager

   For an x86-64 VM use `GOARCH=amd64` instead.

2. **Install Tailscale** on the host and join it to your tailnet (`curl -fsSL https://tailscale.com/install.sh | sh`, then `sudo tailscale up`). In the Tailscale admin console, enable MagicDNS and HTTPS certificates.

3. **Create a service user** on the host:

       sudo useradd --system --no-create-home --shell /usr/sbin/nologin moneymanager

4. **Copy the binary and the unit** to the host and install them:

       scp moneymanager deploy/moneymanager.service <host>:
       ssh <host>
       sudo install -m 0755 moneymanager /usr/local/bin/moneymanager
       sudo install -m 0644 moneymanager.service /etc/systemd/system/moneymanager.service

5. **Start the app** and enable it on boot. systemd restarts it on failure:

       sudo systemctl daemon-reload
       sudo systemctl enable --now moneymanager
       systemctl status moneymanager

   The database lives at `/var/lib/moneymanager/money_manager.db` (set by `-db` in the unit); it is created and migrated on first start.

6. **Make it reachable over Tailscale** (persists across reboots):

       sudo tailscale serve --bg 47613

   Open `https://<host>.<tailnet>.ts.net` on a device in your tailnet. `tailscale serve status` shows what is served.

**Updating** (no continuous deployment, ADR 0005): build as in step 1, copy the binary over, then:

    sudo install -m 0755 moneymanager /usr/local/bin/moneymanager
    sudo systemctl restart moneymanager

Logs: `journalctl -u moneymanager`.

## Known limitations

- Search only ignores case for ASCII letters: "a" matches "A", but "ä" does not match "Ä".