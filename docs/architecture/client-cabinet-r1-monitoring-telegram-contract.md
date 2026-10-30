# R1 monitoring and Telegram notification contract

Status: accepted implementation contract, 2026-09-22. Owners: Monitoring
collects/aggregates state; Notifications owns recipients, outbox and delivery.
Neither module may read arbitrary project/customer content.

## Time and scheduling

- Technical samples are due on quarter-hour boundaries in `Europe/Moscow`:
  `:00`, `:15`, `:30`, `:45`. The stored timestamp is UTC and the logical slot
  has a unique key, so a restart cannot create a second sample for that slot.
- A daily report period is the half-open interval `[previous local 00:00,
  current local 00:00)` in the IANA zone `Europe/Moscow`, converted to UTC once
  when the report job is created. `BETWEEN` is not used.
- The daily job is due at 09:00 `Europe/Moscow`. On restart it catches up every
  missing period from the last seven days, oldest first. Older gaps require an
  explicit operator backfill and are not silently skipped.
- One transaction-scoped PostgreSQL advisory lock protects sample/report
  scheduling. Uniqueness remains the correctness boundary; the lock only avoids
  duplicate work.
- All percentile formulas use the samples inside the report interval. The
  message states `received/expected` sample coverage; for a 24-hour day the
  normal value is `96/96`. Missing data is shown as `n/a`, never as zero.

## Technical sample schema

One 15-minute sample contains only numeric/status aggregates:

| Area | Values |
| --- | --- |
| VM | uptime seconds; CPU busy %; RAM working-set %/bytes; swap used bytes; disk used %/bytes; I/O wait %; load 1/5/15; network RX/TX byte counters |
| Containers | expected/running/healthy counts; per service health enum and cumulative restart count; release version |
| HTTP/API | request count by route template/status class; p50/p95/p99 latency; 5xx count; Go goroutines; outbox pending/failed counts |
| PostgreSQL | server availability; database size; active/max connections; lock wait/deadlock count; transaction/rollback count; bounded slow-query count |
| Backup | last successful local/off-site completion, verified checksum/upload flags, size bytes and most recent restore-drill time |

Metrics labels are a closed allowlist: service name, route template, status
class, result code and release version. They must not contain a raw URL/query,
login, email, name, IP, user/project ID, chat ID, token, cookie, SQL text,
project title/address, file name or message content.

## Daily aggregate definitions

All database counts use the same transaction snapshot after the report period
has closed.

| Output | Exact definition |
| --- | --- |
| Accounts total | `users.created_at < period_end`; includes active, pending and disabled accounts, excludes rows completed by the future legal-erasure procedure before `period_end` |
| New registrations | count of `account.registration_accepted` events in `[start,end)`; retries do not add events |
| Successful logins | count of `auth.login(result=success)` interactive events in `[start,end)` |
| Unique logged-in users | `COUNT(DISTINCT actor_user_id)` over those successful interactive login events |
| Failed logins | count of `auth.login` events with `invalid_credentials`, `unverified`, `disabled` or `rate_limited`; refresh/session restore/CSRF errors are excluded |
| Active sessions at end | sessions created before `end`, not revoked before `end`, with idle and absolute expiry later than `end`; a rotated/replaced row is not active |
| CPU/RAM | average, nearest-rank p95 and maximum of valid samples |
| Disk | first/last used bytes, signed delta, final used %, maximum used % |
| I/O wait | nearest-rank p95 and maximum |
| Containers | any unhealthy slot; restart delta per service from first to last counter; final expected/running/healthy counts |
| PostgreSQL | first/last database bytes and delta; maximum/limit connections; total deadlocks and slow-query count |
| Backup | last successful off-site completion known at report generation, its age and verification flags; no bucket/path/credential |

The report carries `report_id=daily:<period_start-date>:<recipient_user_id>`
internally. The Telegram text shows only the local date and aggregate sections;
recipient IDs and chat IDs are transport metadata and are not included in the
message body.

## Recipient resolution and payload

Recipients are resolved in the enqueue transaction at send time. A recipient
must simultaneously have `users.status=active`, `global_role=super_admin` and
an active, verified, private Telegram subscription linked to that database
user. No hard-coded `chat_id` or environment admin credential participates in
recipient selection. Disable, unsubscribe or role removal before enqueue means
no message; before a retry it moves the delivery to terminal `recipient_inactive`.

Daily message example structure (values only, no hidden detail):

```text
Сводка arhDesign за 21.09.2026 (МСК)
Данные мониторинга: 96/96 интервалов
Система: CPU avg/p95/max …; RAM …; диск …; I/O wait …
Контейнеры: 5/5 healthy; перезапуски 0
PostgreSQL: размер … (Δ …); соединения max …/…; deadlocks …
Backup: off-site OK, возраст …; restore drill …
Пользователи: всего …; регистрации …
Входы: успешные …; уникальные …; неуспешные …; активные сессии …
```

If coverage is below 90%, backup state is unknown, or a collector failed, the
summary starts with `ДАННЫЕ НЕПОЛНЫЕ` and names only the failed subsystem.

## Alert state machine

An incident key is `(type, resource_key)` with states `normal → pending →
active → recovering → resolved`. Exactly one alert row exists for an active
incident. A repeated qualifying sample updates peak/last-seen data but does not
enqueue a second “problem” message. Recovery emits one message and closes the
incident.

Default thresholds (all are configurable but cannot be silently disabled in
production):

| Incident | Activate | Recover |
| --- | --- | --- |
| CPU high | `>85%` in two consecutive samples or `>95%` once | `<80%` in two consecutive samples |
| RAM high | `>85%` in two consecutive samples or `>95%` once | `<80%` in two consecutive samples |
| Disk critical | `>=80%` once (`>=70%` is dashboard warning only) | `<75%` once |
| I/O wait high | p95 `>20%` in two consecutive samples | `<10%` in two consecutive samples |
| PostgreSQL connections | `>=85%` of configured maximum twice or `>=95%` once | `<70%` twice |
| Backup stale/failed | no verified successful off-site backup for `>26 h`, or latest attempt checksum/upload failed | verified success with age `<=26 h` |
| Container unhealthy | expected service unhealthy/missing once | healthy in two consecutive observations |
| Container restart | restart counter increases unexpectedly | recovery after two healthy observations; planned deploy window suppresses only the known release restart |
| Failed-login spike | at least 50 failed interactive logins in 15 min, or at least 20 and `>=3×` the median matching 15-min slot over the prior 7 days | below both triggers for two intervals |
| Metrics collector | two consecutive missed/failed slots | two consecutive complete slots |

Alert text contains incident type, affected service category, first/last local
time, current/threshold numeric value, release version and a runbook key. It
does not contain the account, identifier, IP, project, request body or stack
trace that contributed to the aggregate.

## Outbox, idempotency and delivery truth

The database guarantees exactly one logical daily delivery row per
`(report_type, period_start, period_end, recipient_user_id)` and one problem/
recovery row per incident transition. Scheduler retries and restarts therefore
cannot create a second logical report.

Telegram Bot API does not accept an application idempotency key. Consequently,
the external network boundary cannot honestly guarantee exactly-once delivery
if Telegram accepted a message but the caller lost the response. R1 uses
at-least-once delivery for retryable or ambiguous failures; a rare duplicate is
possible and carries the same visible period/incident key. This is preferred to
silently losing a daily report or critical alert. Known success is never sent
again.

Retry schedule: 30 seconds, 2 minutes, 10 minutes, 30 minutes, 2 hours, then
every 6 hours until 24 hours after creation. `4xx` authentication/chat errors
are terminal; `429` honors Telegram `retry_after`; timeout, `5xx` and relay
unavailability are retryable. HTTP auth/project flows never wait for Telegram.
Outbox payload and errors are allowlisted and contain no secrets or PII.

The required independent infrastructure alert channel from the backup runbook
remains necessary: the application cannot report its own complete VM/DB outage
through the same failed VM/database/bot path.

## Configuration contract

Production configuration exposes validated values for timezone, sample
interval (minimum 15 minutes in R1), report hour, thresholds, expected service
names and delivery retry ceiling. Startup fails on an invalid timezone,
non-positive window, report hour outside `00..23`, missing recipient linkage or
threshold ordering where recovery is not below activation.

Changing a threshold produces an audit event with actor, old/new numeric value
and effective time, but never a secret. Configuration source is runtime env or
an admin-owned table added by a later version; R1 has no unauthenticated control
endpoint for monitoring settings.

## Required verification

- Unit tests use an injected clock around local midnight, month/year boundary,
  leap day and scheduler restart; `[start,end)` has no overlap/gap.
- Aggregate fixtures distinguish interactive login from refresh and prove the
  exact failed-login result set and historical active-session formula.
- Percentile, disk delta, counter reset, missing sample and `n/a` behavior are
  deterministic.
- PostgreSQL integration races two schedulers and proves one logical delivery;
  crash/restart reuses it and recipient status is rechecked.
- Alert tests cover one-sample/two-sample thresholds, hysteresis, deduplication,
  recovery, planned restart suppression and 7-day baseline minimum.
- Payload snapshot tests fail if login, email, name, IP, user/project/chat ID,
  token, cookie, project text or raw error sentinel appears.
- Telegram adapter tests cover success, timeout/ambiguous retry, `429
  retry_after`, terminal `4xx`, disabled recipient and the documented possible
  duplicate at the external boundary.

