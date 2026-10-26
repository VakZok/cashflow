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

The Go version is pinned in `go.mod` (see ADR 0005). CI (`.github/workflows/ci.yml`) runs on pushes to `main` and `integration/**`, on pull requests, and weekly (Mondays 06:00 UTC) so new vulnerability advisories surface without code changes. Separate jobs: style (golangci-lint), tests (-race), security (govulncheck), deps (`go mod tidy` must leave `go.mod`/`go.sum` unchanged, then `go mod verify`), shell (shellcheck on `deploy/*.sh`) and a Pi build, which runs last. If the weekly run fails, CI opens a `needs-triage` issue (or comments on the open one). Build for the Raspberry Pi 4 (Pi OS 64-bit):

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

## Nightly backup to the NAS

Every night at 03:00 a systemd timer runs `deploy/moneymanager-backup.sh` outside the app. It takes a consistent copy of the database with SQLite's online backup, checks it with `PRAGMA integrity_check` (anything but `ok` fails the run and nothing is sent), keeps it on the host as `finances_YYYY_MM_DD.db` (a second run the same day replaces that day's file) and pushes the whole local folder to the NAS with rsync over SSH, without `--delete`. Every copy is kept, on the host and on the NAS. Nights missed while the NAS was off arrive with the next successful run; a run missed while the host was off happens at the next boot. If the NAS is unreachable the run fails after a short SSH timeout, which is expected. As its very last step, a fully successful run pings [healthchecks.io](https://healthchecks.io) with a body-less GET; when the pings stop for about a day (timer off, SSH key changed, NAS gone, host dead), healthchecks.io alerts your phone. A failed run sends no ping, and a failed ping only logs a warning.

Out of scope: everything from the NAS onward (snapshots, encryption at rest, the encrypted offsite copy) is configured on the NAS, not in this project. Restore is handled by the existing restore pipeline.

The commands below assume the app runs as user `moneymanager` with its database at `/var/lib/moneymanager/money_manager.db`; adjust the env file and the service's `User=` if yours differ.

1. Install the SQLite command-line tool, rsync and curl on the host:

   ```sh
   sudo apt install sqlite3 rsync curl
   ```

2. On the TrueNAS, create a dataset for the backups (e.g. `tank/cashflow-backup`) and a dedicated user (e.g. `cashflow-backup`) that owns it and has no other shares or permissions. Enable SSH for that user.

3. On the host, create a dedicated key pair for the service user (no passphrase):

   ```sh
   sudo install -d -m 700 -o moneymanager -g moneymanager /etc/moneymanager
   sudo -u moneymanager ssh-keygen -t ed25519 -N '' -C cashflow-backup -f /etc/moneymanager/backup_ed25519
   ```

   Add the public key (`/etc/moneymanager/backup_ed25519.pub`) to the NAS user's authorized keys, restricted so it can only rsync into the dataset:

   ```
   command="rrsync -wo /mnt/tank/cashflow-backup",restrict ssh-ed25519 AAAA... cashflow-backup
   ```

   `-wo` makes the key write-only. Then connect once as the service user to accept the NAS host key (rrsync rejects the session, that is fine):

   ```sh
   sudo -u moneymanager ssh -i /etc/moneymanager/backup_ed25519 cashflow-backup@truenas.local
   ```

4. Create the local backup folder and the env file:

   ```sh
   sudo install -d -m 700 -o moneymanager -g moneymanager /var/backups/moneymanager
   sudo install -m 640 -o root -g moneymanager deploy/backup.env.example /etc/moneymanager/backup.env
   sudoedit /etc/moneymanager/backup.env
   ```

   The real env file stays on the host and is never committed. `PING_URL` is required (see the next step); without it the run fails at the start.

5. Create the check on healthchecks.io:

   - Sign up at [healthchecks.io](https://healthchecks.io) and add a check with a neutral name such as `nightly-job`, never one that says what the host does.
   - Set the period to 1 day and the grace time to about 1 day.
   - Under Integrations, add a channel that reaches your phone (the healthchecks.io mobile app, ntfy, Pushover, Telegram, Signal or email) and enable it for the check.
   - Copy the check's ping URL (`https://hc-ping.com/<uuid>`) into `PING_URL` in `/etc/moneymanager/backup.env`. Keep it only there: whoever has it can send fake "all good" pings.

   The ping carries no data; healthchecks.io only sees the home's public IP, the ping times and the check's name. To switch to a self-hosted Healthchecks later, change only `PING_URL` (`http://` works too). A night with the NAS off triggers an alert; the check recovers on the next successful run.

6. Install the script and units, then enable the timer:

   ```sh
   sudo install -m 755 deploy/moneymanager-backup.sh /usr/local/bin/moneymanager-backup
   sudo install -m 644 deploy/moneymanager-backup.service deploy/moneymanager-backup.timer /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now moneymanager-backup.timer
   ```

7. Run it once by hand and check the result:

   ```sh
   sudo systemctl start moneymanager-backup.service
   journalctl -u moneymanager-backup.service -n 20
   systemctl list-timers moneymanager-backup.timer
   ```

   Then check the catch-up: shut the NAS down, run it (it fails, the day's copy stays on the host), start the NAS, run it again and see the copy arrive.

   Check the alert path: after the run, the check on healthchecks.io shows the ping arrive. Then stop the timer (`sudo systemctl stop moneymanager-backup.timer`), wait out period plus grace, see the alert on your phone, and start the timer again.

## Known limitations

- Search only ignores case for ASCII letters: "a" matches "A", but "ä" does not match "Ä".