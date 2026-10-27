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

## Run

```zsh
go build -o cashflow ./cmd/cashflow
./cashflow -db test.db
```

It serves on http://127.0.0.1:47613.

The SQLite database is created and migrated on startup.

Dummy data for trying out the Report (refuses a non-empty database):

```zsh
go run ./cmd/seed -db test.db -months 3
```

`cmd/import` is a one-time migration of an older `finances.db` (Einnahme/Ausgabe rows) into a new database, not part of the app. It writes nothing if any row breaks the app's rules and lists each problem instead:

```zsh
go run ./cmd/import -from finances.db -db new_finances.db
```

## Deployment

**Never expose the app publicly**; it has no login. It listens on `127.0.0.1` only; how you reach it is up to you.

The app runs as your own user from your clone; if you want isolation, set up a dedicated service user yourself. Replace `/path/to/cashflow` below and in `deploy/cashflow.service` with the path of your clone.

1. **Install Go** 1.26.5 or newer (the version in `go.mod`).

2. **Build** in the clone:

       cd /path/to/cashflow
       go build -o cashflow ./cmd/cashflow

3. **Install the user unit**:

       mkdir -p ~/.config/systemd/user
       cp /path/to/cashflow/deploy/cashflow.service ~/.config/systemd/user/

4. **Start the app** and enable it at boot. systemd restarts it on failure:

       systemctl --user daemon-reload
       systemctl --user enable --now cashflow
       sudo loginctl enable-linger "$USER"
       systemctl --user status cashflow

   The database is `/path/to/cashflow/finances.db`; it is created and migrated on first start.

Logs: `journalctl --user -u cashflow`.

**Updating**:

    cd /path/to/cashflow
    git pull
    go build -o cashflow ./cmd/cashflow
    systemctl --user restart cashflow

## Nightly backup

Every night at 03:00 (or at next boot, if the machine was off then) a user timer runs `deploy/cashflow-backup.sh`. It takes a consistent copy of the database, checks it with `PRAGMA integrity_check`, keeps it as `/path/to/cashflow/backups/finances_YYYY_MM_DD.db`, optionally copies the backups folder to an rclone remote you configured yourself (nothing is ever deleted there), and pings [healthchecks.io](https://healthchecks.io) only after a fully successful run.

1. **Install the tools**:

       sudo apt install sqlite3 curl rclone

2. **Create the env file** from the example and fill it in. `PING_URL` is required; set `RCLONE_DEST` (e.g. `myremote:cashflow`) to push the backups:

       cp /path/to/cashflow/deploy/backup.env.example /path/to/cashflow/backup.env
       chmod 600 /path/to/cashflow/backup.env

3. **Set up healthchecks.io**: add a check with a neutral name such as `nightly-job`, period 1 day, grace about 1 day. Under Integrations, add a channel that reaches your phone and enable it for the check. Put the check's ping URL into `PING_URL` in `backup.env` and nowhere else.

4. **Install the user units** (replace `/path/to/cashflow` in `deploy/cashflow-backup.service` first) and enable the timer:

       cp /path/to/cashflow/deploy/cashflow-backup.service /path/to/cashflow/deploy/cashflow-backup.timer ~/.config/systemd/user/
       systemctl --user daemon-reload
       systemctl --user enable --now cashflow-backup.timer

5. **Run it once** and check the result:

       systemctl --user start cashflow-backup
       journalctl --user -u cashflow-backup -n 20

To test the alert, stop the timer (`systemctl --user stop cashflow-backup.timer`), wait out period plus grace, see the alert on your phone, and start the timer again.

## Known limitations

- Search only ignores case for ASCII letters: "a" matches "A", but "ä" does not match "Ä".