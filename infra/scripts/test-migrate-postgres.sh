#!/bin/sh
set -eu

# Use an isolated PostgreSQL test server. This script creates temporary databases.
root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
runner="$root/infra/scripts/migrate-postgres.sh"
migrations="$root/apps/api/migrations"
workdir=$(mktemp -d)
fresh_db="arhdesign_test_migrate_fresh_$$"
legacy_db="arhdesign_test_migrate_legacy_$$"
set -- "$migrations"/*.sql
expected_count=$#

cleanup() {
  result=$?
  if [ "$result" -ne 0 ]; then
    for log in "$workdir"/*.log; do
      if [ -f "$log" ]; then tail -30 "$log"; fi
    done
  fi
  dropdb --maintenance-db=postgres --if-exists "$fresh_db"
  dropdb --maintenance-db=postgres --if-exists "$legacy_db"
  rm -rf "$workdir"
}
trap cleanup EXIT

createdb --maintenance-db=postgres "$fresh_db"
createdb --maintenance-db=postgres "$legacy_db"

PGDATABASE=$fresh_db sh "$runner" "$migrations" > "$workdir/fresh.log" 2>&1 &
first_pid=$!
PGDATABASE=$fresh_db sh "$runner" "$migrations" > "$workdir/concurrent.log" 2>&1
wait "$first_pid"
test "$(psql -XAtd "$fresh_db" -c 'SELECT count(*) FROM arhdesign_schema_migrations')" = "$expected_count"

mkdir "$workdir/migrations"
cp "$migrations"/*.sql "$workdir/migrations/"
cat > "$workdir/migrations/999_fail.sql" <<'SQL'
BEGIN;
CREATE TABLE rollback_probe (id INTEGER);
SELECT 1 / 0;
COMMIT;
SQL
if PGDATABASE=$fresh_db sh "$runner" "$workdir/migrations" > "$workdir/rollback.log" 2>&1; then
  echo 'A failing migration must stop the runner' >&2
  exit 1
fi
grep -q 'division by zero' "$workdir/rollback.log"
test "$(psql -XAtd "$fresh_db" -c "SELECT to_regclass('rollback_probe') IS NULL AND NOT EXISTS (SELECT FROM arhdesign_schema_migrations WHERE version = '999_fail.sql')")" = t

for migration in "$migrations"/00*.sql "$migrations"/01*.sql "$migrations"/020_*.sql; do
  psql -Xqd "$legacy_db" -v ON_ERROR_STOP=1 -f "$migration"
done
if PGDATABASE=$legacy_db sh "$runner" "$migrations" > "$workdir/partial.log" 2>&1; then
  echo 'An incomplete legacy schema must not be baselined' >&2
  exit 1
fi
grep -q 'Cannot baseline an existing schema without migration 021' "$workdir/partial.log"
test "$(psql -XAtd "$legacy_db" -c "SELECT to_regclass('arhdesign_schema_migrations') IS NULL")" = t
psql -Xqd "$legacy_db" -v ON_ERROR_STOP=1 -f "$migrations/021_add_document_context_chats.sql"

psql -Xqd "$legacy_db" -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO users (id, login, login_normalized, email, email_normalized, status, email_verified_at)
SELECT ('00000000-0000-0000-0000-00000000000' || n)::UUID, login, login,
       login || '@example.com', login || '@example.com', 'active', now()
FROM (VALUES (1, 'owner'), (2, 'removed'), (3, 'uninvited')) AS fixture(n, login);

INSERT INTO projects (id, created_by_user_id, name, type)
SELECT '10000000-0000-0000-0000-000000000001', id, 'Test project', 'test'
FROM users WHERE login = 'owner';

INSERT INTO project_memberships (project_id, user_id, project_role)
SELECT projects.id, users.id, 'executor' FROM projects CROSS JOIN users;

INSERT INTO chats (id, project_id, kind, name, created_by_user_id)
SELECT '20000000-0000-0000-0000-000000000001', id, 'project', 'Private chat', created_by_user_id
FROM projects;

INSERT INTO chat_memberships (chat_id, project_id, user_id, added_by_user_id, removed_at, removed_by_user_id)
SELECT chats.id, chats.project_id, users.id, chats.created_by_user_id,
       CASE WHEN login = 'removed' THEN now() END,
       CASE WHEN login = 'removed' THEN chats.created_by_user_id END
FROM chats CROSS JOIN users WHERE login IN ('owner', 'removed');
SQL

for attempt in upgrade repeat; do
  PGDATABASE=$legacy_db sh "$runner" "$migrations" > "$workdir/$attempt.log" 2>&1
  test "$(psql -XAtd "$legacy_db" -c "SELECT string_agg(login, ',' ORDER BY login) FROM chat_memberships JOIN users ON users.id = user_id WHERE removed_at IS NULL")" = owner
  test "$(psql -XAtd "$legacy_db" -c "SELECT to_regclass('chat_images') IS NOT NULL AND (SELECT count(*) FROM arhdesign_schema_migrations) = $expected_count")" = t
done
grep -q '^Applying 022_create_chat_images.sql$' "$workdir/upgrade.log"
if grep -q '^Applying ' "$workdir/repeat.log"; then
  echo 'A repeat deployment applied an existing migration' >&2
  exit 1
fi
