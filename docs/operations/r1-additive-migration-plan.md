# R1 additive migration and recovery plan

Status: implemented and locally verified on PostgreSQL 17.11 for
R1-DATA-004—005. Migrations:
`005_create_r1_identity_sessions_projects.sql`,
`006_create_r1_audit_monitoring_delivery_ledgers.sql` and
`007_add_notification_outbox_channel_due_index.sql` and
`008_add_notification_outbox_stale_delivery_index.sql`.

## Source and target

Source is the current `0.1.0` schema after migrations `001`–`004`: contact
submissions/cooldowns, Telegram subscribers, consent audit and Telegram
notification receipts. Target adds identity, profiles/roles, credentials,
opaque sessions, verification/reset tokens, durable rate-limit buckets,
projects/memberships, append-only audit, encrypted notification outbox,
monitoring samples/incidents and the daily report ledger. Existing tables and
columns are not renamed or removed; legacy Telegram rows remain valid with
nullable account linkage.

The migration runs as one transaction. A SQL error rolls back all R1 objects and
the Telegram column changes together. The migration is idempotent because the
current Compose runner replays every SQL file on each deployment.

## Table decisions

| Table/change | Consumer and invariant | Expected R1 growth / deletion |
| --- | --- | --- |
| `professional_roles` | registration role selector; stable code unique, active flag | tens; deactivate, do not delete in-use role |
| `users`, `profiles`, `credentials` | Identity owner; unique normalized login/email; one profile/credential per user | up to 10k; disable first, legal erasure is a separate procedure |
| `sessions` | Session owner; unique opaque access/refresh hashes and version capture | up to 20 active/history rows per user; purge 120 days after terminal state |
| verification/reset tokens | Identity; one active token of each purpose per user, raw value never stored | bounded; purge 30 days after expiry/consume/revoke |
| `auth_rate_limit_buckets` | durable concurrent token bucket keyed by HMAC subject | bounded by recent subjects/policies; purge after 48h inactivity |
| `projects`, `project_memberships` | Projects owner; optimistic version, at most one active customer, customer pointer matches membership at commit | tens per user/project; archive/revoke rather than immediate delete |
| `telegram_subscribers` columns | link verified private subscription to a database user | legacy rows get null linkage and cannot receive R1 account reports until verified |
| `audit_events` | allowlisted security/project history; type/result/time aggregate index | 365-day security retention unless explicit hold; bounded deletion |
| `notification_outbox` | encrypted email/Telegram payload with unique idempotency key | ciphertext 30 days after success, terminal failure 90 days |
| `monitoring_samples`, `monitoring_incidents` | 15-minute technical aggregates and deduplicated alert state | samples 90 days; incidents 365 days after recovery |
| `report_deliveries` | one logical report per type/period/recipient | aggregate payload 365 days; unique key is concurrency boundary |

Money is not introduced in R1. Time instants use `TIMESTAMPTZ`; project planning
uses `DATE`. Passwords use versioned Argon2id PHC text; token material uses
purpose-keyed hashes plus key version.

## Lock and deploy analysis

- Creating new tables/indexes does not lock existing application tables.
- `ALTER TABLE telegram_subscribers ADD COLUMN` briefly takes
  `ACCESS EXCLUSIVE`. Nullable columns are metadata-only; the constant default
  for `version` uses PostgreSQL 17 fast-default behavior rather than rewriting
  every row.
- New checks are added `NOT VALID` and then validated. Validation scans only the
  existing Telegram subscriber table and does not rewrite it. The table is
  currently expected to contain administrator subscriptions, not millions of
  rows; production row count and lock wait must still be captured before deploy.
- The partial unique index on linked active subscribers reads that small table.
  If measured production size or lock wait is unexpectedly large, split it into
  a pre-deploy `CREATE UNIQUE INDEX CONCURRENTLY` step outside this transaction.
- `lock_timeout`/`statement_timeout` are supplied by the deploy session. A
  timeout fails the migration and leaves the old schema intact; deploy does not
  start the new API.

Compatibility matrix:

| Application | Schema 001–004 | Schema 001–006 |
| --- | --- | --- |
| old `0.1.x` | supported | supported; ignores additive objects/columns/indexes |
| R1 `0.2.0` | must fail readiness before serving account routes | supported |

Deployment order is migration → R1 API → R1 web. Rolling the application back
to `0.1.x` is safe without removing the additive schema.

## Recovery

There is deliberately no destructive down migration. Before deployment create
and verify the release backup required by `postgresql-backup-and-recovery.md`.

1. If migration fails, its transaction rolls back; fix forward and rerun.
2. If migration succeeds but R1 application fails, redeploy `0.1.x`; preserve
   R1 tables for diagnosis and retry.
3. If bad R1 writes must be discarded before production use, take an incident
   copy, verify that no R1 data must be retained, then use a separately reviewed
   cleanup migration. Do not issue ad-hoc `DROP ... CASCADE` in production.
4. If existing data was damaged (not expected from this additive migration),
   restore the verified pre-deploy backup into a new PostgreSQL instance,
   validate it, and switch over; never overwrite the only production volume.

## Verification evidence and required CI

Integration tests create isolated schemas and cover:

- migrations `001`–`008` on an empty PostgreSQL 17 schema;
- a second complete replay for idempotency;
- `001`–`004` with legacy contact/consent/Telegram rows, followed by `005–006`,
  proving those rows survive and nullable linkage defaults are correct;
- four seeded professional roles;
- a valid deferred project/customer transaction and rejection of an
  inconsistent customer pointer at commit.
- a 20-writer race on the daily report logical key with exactly one database
  winner and one persisted row.

Locally verified on PostgreSQL `17.11` with:

```sh
go test -tags=integration -count=1 ./internal/repository
```

It must run only against an isolated database whose name contains `test`.
Before PR, CI must additionally record PostgreSQL version, source table row
counts, migration duration, lock waits and empty/upgrade test result.
