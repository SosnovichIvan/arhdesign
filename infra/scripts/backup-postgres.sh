#!/bin/sh
set -eu

positive_integer() {
  name="$1"
  value="$2"
  case "$value" in
    ''|*[!0-9]*) echo "$name must be a positive integer" >&2; exit 1 ;;
  esac
  if [ "$value" -lt 1 ]; then
    echo "$name must be a positive integer" >&2
    exit 1
  fi
}

load_secret_file() {
  name="$1"
  file_name="${name}_FILE"
  eval "file_path=\${$file_name:-}"
  if [ -n "$file_path" ]; then
    if [ ! -r "$file_path" ]; then
      echo "$file_name is not readable" >&2
      exit 1
    fi
    value="$(sed -n '1p' "$file_path")"
    export "$name=$value"
  fi
}

local_retention_days="${BACKUP_LOCAL_RETENTION_DAYS:-7}"
interval_seconds="${BACKUP_INTERVAL_SECONDS:-86400}"
positive_integer BACKUP_LOCAL_RETENTION_DAYS "$local_retention_days"
positive_integer BACKUP_INTERVAL_SECONDS "$interval_seconds"
local_retention_threshold=$((local_retention_days - 1))

load_secret_file RESTIC_PASSWORD
load_secret_file AWS_ACCESS_KEY_ID
load_secret_file AWS_SECRET_ACCESS_KEY

if [ "${OFFSITE_BACKUP_ENABLED:-false}" = "true" ]; then
  : "${RESTIC_REPOSITORY:?RESTIC_REPOSITORY is required when off-site backup is enabled}"
  : "${RESTIC_PASSWORD:?RESTIC_PASSWORD or RESTIC_PASSWORD_FILE is required}"
  case "$RESTIC_REPOSITORY" in
    s3:*) ;;
    *)
      if [ "${ALLOW_LOCAL_RESTIC_REPOSITORY:-false}" != "true" ]; then
        echo "RESTIC_REPOSITORY must be an s3: repository for production off-site backups" >&2
        exit 1
      fi
      ;;
  esac
fi

mkdir -p /backups

run_backup() {
  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  pending_path="/backups/.arhdesign-${timestamp}.dump.pending"
  final_path="/backups/arhdesign-${timestamp}.dump"
  checksum_path="${final_path}.sha256"

  pg_dump --format=custom --compress=9 --no-owner --no-privileges --file="$pending_path"
  test -s "$pending_path"
  mv "$pending_path" "$final_path"
  (cd /backups && sha256sum "$(basename "$final_path")" > "$(basename "$checksum_path")")

  if [ "${OFFSITE_BACKUP_ENABLED:-false}" = "true" ]; then
    if ! restic snapshots >/dev/null 2>&1; then
      restic init
    fi
    restic backup "$final_path" "$checksum_path" --host "${BACKUP_HOST_ID:-arhdesign}" --tag postgres --tag daily
    restic forget --host "${BACKUP_HOST_ID:-arhdesign}" --tag postgres --keep-daily "${BACKUP_REMOTE_DAILY_RETENTION:-30}" --keep-monthly "${BACKUP_REMOTE_MONTHLY_RETENTION:-12}" --prune
    restic check --read-data-subset="${BACKUP_CHECK_SUBSET:-1/30}"
    printf '%s\n' "$timestamp" > /backups/.latest-offsite-success.pending
    mv /backups/.latest-offsite-success.pending /backups/latest-offsite-success
  fi

  find /backups -type f \( -name 'arhdesign-*.dump' -o -name 'arhdesign-*.dump.sha256' \) -mtime "+$local_retention_threshold" -delete
  echo "backup completed: $(basename "$final_path")"
}

while true; do
  run_backup
  if [ "${BACKUP_RUN_ONCE:-false}" = "true" ]; then
    break
  fi
  sleep "$interval_seconds"
done
