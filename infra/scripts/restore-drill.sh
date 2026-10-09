#!/bin/sh
set -eu

: "${RESTIC_REPOSITORY:?RESTIC_REPOSITORY is required}"
: "${RESTORE_DATABASE_URL:?RESTORE_DATABASE_URL is required}"
: "${RESTORE_ACKNOWLEDGE_ISOLATED_TARGET:?Set RESTORE_ACKNOWLEDGE_ISOLATED_TARGET=yes for an isolated empty PostgreSQL target}"

if [ "$RESTORE_ACKNOWLEDGE_ISOLATED_TARGET" != "yes" ]; then
  echo "Restore drill refused: target was not acknowledged as isolated" >&2
  exit 1
fi

load_secret_file() {
  name="$1"
  file_name="${name}_FILE"
  eval "file_path=\${$file_name:-}"
  if [ -n "$file_path" ]; then
    test -r "$file_path" || { echo "$file_name is not readable" >&2; exit 1; }
    value="$(sed -n '1p' "$file_path")"
    export "$name=$value"
  fi
}

load_secret_file RESTIC_PASSWORD
load_secret_file AWS_ACCESS_KEY_ID
load_secret_file AWS_SECRET_ACCESS_KEY
: "${RESTIC_PASSWORD:?RESTIC_PASSWORD or RESTIC_PASSWORD_FILE is required}"

started_epoch="$(date +%s)"
restore_root="$(mktemp -d)"
trap 'rm -rf "$restore_root"' EXIT INT TERM

restic check
restic restore latest --host "${BACKUP_HOST_ID:-arhdesign}" --tag postgres --target "$restore_root"
dump_path="$(find "$restore_root" -type f -name 'arhdesign-*.dump' | sort | tail -n 1)"
test -n "$dump_path"
checksum_path="${dump_path}.sha256"
test -f "$checksum_path"
(cd "$(dirname "$dump_path")" && sha256sum -c "$(basename "$checksum_path")")

pg_restore --exit-on-error --no-owner --no-privileges --dbname="$RESTORE_DATABASE_URL" "$dump_path"
table_count="$(psql "$RESTORE_DATABASE_URL" --tuples-only --no-align --command="SELECT count(*) FROM pg_catalog.pg_tables WHERE schemaname = 'public';")"
if [ "$table_count" -lt "${RESTORE_MINIMUM_TABLE_COUNT:-1}" ]; then
  echo "Restore drill failed: only $table_count public tables restored" >&2
  exit 1
fi

finished_epoch="$(date +%s)"
backup_epoch="$(stat -c %Y "$dump_path")"
rpo_seconds=$((started_epoch - backup_epoch))
rto_seconds=$((finished_epoch - started_epoch))
if [ "$rpo_seconds" -gt "${RESTORE_MAX_RPO_SECONDS:-86400}" ]; then
  echo "Restore drill failed RPO: ${rpo_seconds}s" >&2
  exit 1
fi
if [ "$rto_seconds" -gt "${RESTORE_MAX_RTO_SECONDS:-14400}" ]; then
  echo "Restore drill failed RTO: ${rto_seconds}s" >&2
  exit 1
fi

printf '{"backup":"%s","public_tables":%s,"rpo_seconds":%s,"rto_seconds":%s,"status":"passed"}\n' \
  "$(basename "$dump_path")" "$table_count" "$rpo_seconds" "$rto_seconds"
