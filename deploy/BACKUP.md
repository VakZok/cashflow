# Nightly backup

**Status: untested.** This failed on a Raspberry Pi and hasn't been debugged.

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
