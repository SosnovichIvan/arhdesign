#!/bin/sh
set -eu

export LC_ALL=C
migrations_dir=${1:-/migrations}
set -- "$migrations_dir"/*.sql
if [ ! -f "$1" ]; then
  echo "No migrations found in $migrations_dir" >&2
  exit 1
fi

batch=$(mktemp)
trap 'rm -f "$batch"' EXIT
for migration do
  version=${migration##*/}
  if ! printf '%s\n' "$version" | grep -Eq '^[0-9]{3}_[a-z0-9_]+\.sql$'; then
    echo "Invalid migration filename: $version" >&2
    exit 1
  fi
done

cat > "$batch" <<'SQL'
SELECT pg_advisory_lock(hashtextextended('arhdesign:schema-migrations', 0));
BEGIN;
CREATE TABLE IF NOT EXISTS public.arhdesign_schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

-- Releases through migration 021 used a runner without a ledger. Check the
-- supported legacy schema before recording its history without replaying it.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM public.arhdesign_schema_migrations)
       AND (to_regclass('public.contact_submissions') IS NOT NULL
            OR to_regclass('public.users') IS NOT NULL
            OR to_regclass('public.chats') IS NOT NULL)
       AND (to_regclass('public.account_notifications') IS NULL
            OR to_regclass('public.global_chats') IS NULL
            OR to_regclass('public.chat_memberships') IS NULL
            OR to_regclass('public.project_documents') IS NULL
            OR NOT EXISTS (
                SELECT FROM pg_constraint
                WHERE conrelid = to_regclass('public.chats')
                  AND conname = 'chats_context_type_check'
                  AND strpos(pg_get_constraintdef(oid), '''document''') > 0
            )) THEN
        RAISE EXCEPTION 'Cannot baseline an existing schema without migration 021';
    END IF;
END $$;
SELECT NOT EXISTS (SELECT FROM public.arhdesign_schema_migrations)
       AND to_regclass('public.account_notifications') IS NOT NULL AS baseline_legacy \gset
\if :baseline_legacy
SQL

for migration do
  version=${migration##*/}
  case "$version" in
    00[1-9]_*|01[0-9]_*|020_*|021_*)
      printf "INSERT INTO public.arhdesign_schema_migrations (version) VALUES ('%s');\n" "$version" >> "$batch"
      ;;
  esac
done

cat >> "$batch" <<'SQL'
\endif
COMMIT;
SQL

for migration do
  version=${migration##*/}
  cat >> "$batch" <<SQL
SELECT NOT EXISTS (SELECT FROM public.arhdesign_schema_migrations WHERE version = '$version') AS pending \\gset
\\if :pending
\\echo Applying $version
BEGIN;
SQL
  # Store each version in the same transaction as its SQL, including legacy
  # files whose first and last nonblank lines provide their own transaction.
  awk '
    { lines[NR] = $0 }
    NF { if (!first) first = NR; last = NR }
    END {
      opens = lines[first] ~ /^[[:space:]]*BEGIN;[[:space:]]*$/
      closes = lines[last] ~ /^[[:space:]]*COMMIT;[[:space:]]*$/
      if (!first || opens != closes) exit 1
      for (i = 1; i <= NR; i++) {
        if (!opens || (i != first && i != last)) print lines[i]
      }
    }
  ' "$migration" >> "$batch" || {
    echo "Invalid SQL transaction wrapper in $version" >&2
    exit 1
  }
  cat >> "$batch" <<SQL
INSERT INTO public.arhdesign_schema_migrations (version) VALUES ('$version');
COMMIT;
\\endif
SQL
done

psql -X --set=ON_ERROR_STOP=1 --file="$batch"
