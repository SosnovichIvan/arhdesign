#!/bin/sh
set -eu

retention_days="${BACKUP_RETENTION_DAYS:-30}"
case "$retention_days" in
  ''|*[!0-9]*)
    echo "BACKUP_RETENTION_DAYS must be a positive integer" >&2
    exit 1
    ;;
esac
if [ "$retention_days" -lt 1 ]; then
  echo "BACKUP_RETENTION_DAYS must be a positive integer" >&2
  exit 1
fi
retention_threshold=$((retention_days - 1))

mkdir -p /backups

while true; do
  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  pending_path="/backups/.arhdesign-${timestamp}.sql.gz.pending"
  final_path="/backups/arhdesign-${timestamp}.sql.gz"

  pg_dump --no-owner --no-privileges | gzip -9 > "$pending_path"
  mv "$pending_path" "$final_path"
  find /backups -type f -name 'arhdesign-*.sql.gz' -mtime "+$retention_threshold" -delete

  sleep 86400
done
