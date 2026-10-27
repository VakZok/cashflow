#!/usr/bin/env bash
# Nightly backup of the Cashflow database. Install: see "Nightly backup" in
# README.md.
#
# 1. Take a consistent copy of the live database with SQLite's online backup.
# 2. Check the copy with PRAGMA integrity_check; anything but "ok" fails the run.
# 3. Move it into place as finances_YYYY_MM_DD.db in BACKUP_DIR (created if
#    missing; a same-day rerun replaces that day's file).
# 4. If RCLONE_DEST is set, rclone copy the backup folder there. Copy, never
#    sync: nothing is deleted on the remote.
# 5. Ping PING_URL. Only a fully successful run gets here; when the pings stop,
#    healthchecks.io alerts. A failed ping only warns.
#
# All values come from the environment (the systemd unit loads them from an env
# file, see backup.env.example). Any failure stops the run with a non-zero exit.
set -euo pipefail

: "${DB_PATH:?DB_PATH must be set}"
: "${BACKUP_DIR:?BACKUP_DIR must be set}"
: "${PING_URL:?PING_URL must be set}"
RCLONE_DEST="${RCLONE_DEST:-}"

die() {
	echo "$*" >&2
	exit 1
}

# sqlite3 would silently create an empty database at a wrong path.
[[ -f "$DB_PATH" ]] || die "database not found: $DB_PATH"
# The path goes into an SQL string literal below.
[[ "$BACKUP_DIR" != *"'"* ]] || die "BACKUP_DIR must not contain a single quote"
mkdir -p -- "$BACKUP_DIR"

final="$BACKUP_DIR/finances_$(date +%Y_%m_%d).db"

# Hidden temporary name: a half-written copy never carries a dated name and is
# excluded from the push.
tmp="$(mktemp "$BACKUP_DIR/.finances.XXXXXX.tmp")"
trap 'rm -f -- "$tmp"' EXIT

sqlite3 -bail -readonly "$DB_PATH" ".backup '$tmp'"

check="$(sqlite3 -bail -readonly "$tmp" 'PRAGMA integrity_check;')"
if [[ "$check" != "ok" ]]; then
	echo "integrity check failed on the copy, nothing kept or pushed:" >&2
	echo "$check" >&2
	exit 1
fi

chmod 600 "$tmp"
mv -f -- "$tmp" "$final"
echo "copy written: $final"

if [[ -n "$RCLONE_DEST" ]]; then
	# copy, not sync: files on the remote are never deleted.
	rclone copy --exclude '.*' "$BACKUP_DIR" "$RCLONE_DEST"
	echo "copied $BACKUP_DIR to $RCLONE_DEST"
fi

# Dead man's switch: a body-less GET, so no data leaves the machine. A failed
# ping must not fail a good backup; the missing ping makes healthchecks.io alert.
if curl -fsS -o /dev/null -m 10 --retry 3 "$PING_URL"; then
	echo "pinged healthchecks"
else
	echo "warning: healthchecks ping failed, the backup itself succeeded" >&2
fi
