#!/usr/bin/env bash
# Nightly backup of the Cashflow database to the NAS (issue #135, ADR 0003).
#
# 1. Take a consistent copy of the live database with SQLite's online backup.
# 2. Check the copy with PRAGMA integrity_check; anything but "ok" fails the run.
# 3. Move it into place as finances_YYYY_MM_DD.db (a same-day rerun replaces it).
# 4. Push the whole local folder to the NAS with rsync over SSH, never deleting.
# 5. Ping the healthchecks URL (issue #136). Only a fully successful run gets
#    here; when the pings stop, healthchecks.io alerts. A failed ping only warns.
#
# All host-specific values come from the environment (the systemd service loads
# them from an env file, see backup.env.example). Any failure stops the run with
# a non-zero exit.
set -euo pipefail

: "${DB_PATH:?DB_PATH must be set}"
: "${BACKUP_DIR:?BACKUP_DIR must be set}"
: "${NAS_HOST:?NAS_HOST must be set}"
: "${NAS_USER:?NAS_USER must be set}"
: "${NAS_PATH:?NAS_PATH must be set}"
: "${SSH_KEY:?SSH_KEY must be set}"
: "${PING_URL:?PING_URL must be set}"
SSH_CONNECT_TIMEOUT="${SSH_CONNECT_TIMEOUT:-10}"

# sqlite3 would silently create an empty database at a wrong path.
if [[ ! -f "$DB_PATH" ]]; then
	echo "database not found: $DB_PATH" >&2
	exit 1
fi
if [[ ! -d "$BACKUP_DIR" ]]; then
	echo "backup folder not found: $BACKUP_DIR" >&2
	exit 1
fi
if [[ "$PING_URL" != https://* ]]; then
	echo "PING_URL must start with https://" >&2
	exit 1
fi
# The path goes into an SQL string literal below.
if [[ "$BACKUP_DIR" == *"'"* ]]; then
	echo "BACKUP_DIR must not contain a single quote" >&2
	exit 1
fi

final="$BACKUP_DIR/finances_$(date +%Y_%m_%d).db"

# Hidden temporary name: a half-written copy never carries a dated name and is
# excluded from the push.
tmp="$(mktemp "$BACKUP_DIR/.finances.XXXXXX.tmp")"
trap 'rm -f -- "$tmp"' EXIT

sqlite3 -bail -readonly "$DB_PATH" ".backup '$tmp'"

check="$(sqlite3 -bail -readonly "$tmp" 'PRAGMA integrity_check;')"
if [[ "$check" != "ok" ]]; then
	echo "integrity check failed on the copy, nothing pushed:" >&2
	echo "$check" >&2
	exit 1
fi

chmod 600 "$tmp"
mv -f -- "$tmp" "$final"
echo "copy written: $final"

ssh_cmd="ssh -i $SSH_KEY -o BatchMode=yes -o ConnectTimeout=$SSH_CONNECT_TIMEOUT -o ServerAliveInterval=15 -o ServerAliveCountMax=4"

# No --delete: files on the NAS are never removed by the host.
rsync -rt --timeout=120 --exclude='.*' -e "$ssh_cmd" \
	"$BACKUP_DIR/" "$NAS_USER@$NAS_HOST:$NAS_PATH/"
echo "pushed $BACKUP_DIR to $NAS_HOST"

# Dead man's switch: a body-less GET, so no data leaves the host. A failed ping
# must not fail a good backup; the missing ping makes healthchecks.io alert.
if curl -fsS -o /dev/null -m 10 --retry 3 "$PING_URL"; then
	echo "pinged healthchecks"
else
	echo "warning: healthchecks ping failed, the backup itself succeeded" >&2
fi
