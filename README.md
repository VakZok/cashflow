# Cashflow

Personal finance manager. Purposefully simple.

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

Given 100€ investment:

- sold with gain of 20€ -> 100€ divestment, 15€ income, 15€ levy
- sold with loss of 20€ -> 100€ divestment, 20€ expense

## Run (compiled binary)

```zsh
./cashflow -addr 127.0.0.1:47613 -db test.db
```

The SQLite database is created and migrated on startup.

Dummy data for trying out the Report (refuses a non-empty database):

```zsh
go run ./cmd/seed -db test.db -months 3
```

## Deployment

**Never expose the app publicly**; it has no login. By default, the app listens on `127.0.0.1`.

1. **Build** on your machine. For a Raspberry Pi 4/5 (64-bit Pi OS):

       CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o cashflow ./cmd/cashflow

   For x86-64 use `GOARCH=amd64` instead.

2. **Create a service user** on the host:

       sudo useradd --system --no-create-home --shell /usr/sbin/nologin cashflow

4. **Copy the binary and the unit** to the host and install them:

       scp cashflow deploy/cashflow.service <host>:
       ssh <host>
       sudo install -m 0755 cashflow /usr/local/bin/cashflow
       sudo install -m 0644 cashflow.service /etc/systemd/system/cashflow.service

5. **Start the app** and enable it on boot. systemd restarts it on failure:

       sudo systemctl daemon-reload
       sudo systemctl enable --now cashflow
       systemctl status cashflow

   The database lives at `/var/lib/cashflow/money_manager.db` (set by `-db` in the unit); it is created and migrated on first start.

**Updating** (no continuous deployment, ADR 0005): build as in step 1, copy the binary over, then:

    sudo install -m 0755 cashflow /usr/local/bin/cashflow
    sudo systemctl restart cashflow

Logs: `journalctl -u cashflow`.

## Nightly backup to the NAS

Every night at 03:00 a systemd timer runs `deploy/cashflow-backup.sh` outside the app. It takes a consistent copy of the database with SQLite's online backup, checks it with `PRAGMA integrity_check` (anything but `ok` fails the run and nothing is sent), keeps it on the host as `finances_YYYY_MM_DD.db` (a second run the same day replaces that day's file) and pushes the whole local folder to the NAS with rsync over SSH, without `--delete`. Every copy is kept, on the host and on the NAS. Nights missed while the NAS was off arrive with the next successful run; a run missed while the host was off happens at the next boot. If the NAS is unreachable the run fails after a short SSH timeout, which is expected. As its very last step, a fully successful run pings [healthchecks.io](https://healthchecks.io) with a body-less GET; when the pings stop for about a day (timer off, SSH key changed, NAS gone, host dead), healthchecks.io alerts your phone. A failed run sends no ping, and a failed ping only logs a warning.

Out of scope: everything from the NAS onward (snapshots, encryption at rest, restoration).

The commands below assume the app runs as user `cashflow` with its database at `/var/lib/cashflow/money_manager.db`; adjust the env file and the service's `User=`, `Group=` and `ReadWritePaths=` if yours differ. The backup service is hardened like the app's: it can only write the backup folder and the database folder.

1. Install the SQLite command-line tool, rsync and curl on the host:

       sudo apt install sqlite3 rsync curl

2. On the TrueNAS, create a dataset for the backups (e.g. `tank/cashflow-backup`) and a dedicated user (e.g. `cashflow-backup`) that owns it and has no other shares or permissions. Enable SSH for that user.

3. On the host, create the config folder and a dedicated key pair for the service user (no passphrase). The folder belongs to root and is only readable by the `cashflow` group, so the service can read its config but not change it; only the private key belongs to the service user, because ssh needs it readable by that user alone:

       sudo install -d -m 750 -o root -g cashflow /etc/cashflow
       sudo ssh-keygen -t ed25519 -N '' -C cashflow-backup -f /etc/cashflow/backup_ed25519
       sudo chown cashflow:cashflow /etc/cashflow/backup_ed25519

   Add the public key (`/etc/cashflow/backup_ed25519.pub`) to the NAS user's authorized keys, restricted so it can only rsync into the dataset:

       command="rrsync -wo /mnt/tank/cashflow-backup",restrict ssh-ed25519 AAAA... cashflow-backup

   `-wo` makes the key write-only. Then connect once to accept the NAS host key into the system-wide, root-owned known hosts file, which the service reads (the service user has no home folder, and the service cannot see `/home`). Check the fingerprint against the NAS before you type `yes`; rrsync rejects the session, that is fine:

       sudo ssh -o UserKnownHostsFile=/etc/ssh/ssh_known_hosts -i /etc/cashflow/backup_ed25519 cashflow-backup@truenas.local

4. Create the local backup folder and the env file:

       sudo install -d -m 700 -o cashflow -g cashflow /var/backups/cashflow
       sudo install -m 640 -o root -g cashflow deploy/backup.env.example /etc/cashflow/backup.env
       sudoedit /etc/cashflow/backup.env

   The env file is `root:cashflow` with mode 640: the service reads it, only root can change it. The real env file stays on the host and is never committed. `PING_URL` is required (see the next step); without it the run fails at the start.

5. Create the check on healthchecks.io:

   - Sign up at [healthchecks.io](https://healthchecks.io) and add a check with a neutral name such as `nightly-job`, never one that says what the host does.
   - Set the period to 1 day and the grace time to about 1 day.
   - Under Integrations, add a channel that reaches your phone (the healthchecks.io mobile app, ntfy, Pushover, Telegram, Signal or email) and enable it for the check.
   - Copy the check's ping URL (`https://hc-ping.com/<uuid>`) into `PING_URL` in `/etc/cashflow/backup.env`. Keep it only there: whoever has it can send fake "all good" pings.

   The ping carries no data; healthchecks.io only sees the home's public IP, the ping times and the check's name. To switch to a self-hosted Healthchecks later, change only `PING_URL` (`http://` works too). A night with the NAS off triggers an alert; the check recovers on the next successful run.

6. Install the script and units, then enable the timer:

       sudo install -m 755 deploy/cashflow-backup.sh /usr/local/bin/cashflow-backup
       sudo install -m 644 deploy/cashflow-backup.service deploy/cashflow-backup.timer /etc/systemd/system/
       sudo systemctl daemon-reload
       sudo systemctl enable --now cashflow-backup.timer

7. Run it once by hand and check the result:

       sudo systemctl start cashflow-backup.service
       journalctl -u cashflow-backup.service -n 20
       systemctl list-timers cashflow-backup.timer

   Then check the catch-up: shut the NAS down, run it (it fails, the day's copy stays on the host), start the NAS, run it again and see the copy arrive.

   Check the alert path: after the run, the check on healthchecks.io shows the ping arrive. Then stop the timer (`sudo systemctl stop cashflow-backup.timer`), wait out period plus grace, see the alert on your phone, and start the timer again.

## Known limitations

- Search only ignores case for ASCII letters: "a" matches "A", but "ä" does not match "Ä".