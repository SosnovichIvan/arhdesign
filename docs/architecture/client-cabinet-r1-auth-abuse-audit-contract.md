# R1 auth abuse, audit and revocation contract

Status: accepted implementation contract, 2026-09-22. This document refines
R1-API-003 and is subordinate to the password/session boundary in
`client-cabinet-r1-auth-security-adr.md`.

## Rate-limit model

R1 uses PostgreSQL-backed token buckets so restart or a second API instance does
not reset protection. A consume operation is one atomic `INSERT ... ON CONFLICT
... DO UPDATE ... WHERE tokens >= 1 RETURNING`; the request proceeds only when a
row is returned. Buckets are keyed by `policy_code + HMAC(subject)` and never by
raw IP, login or email.

The API accepts the client address only from Caddy's trusted proxy network. A
forwarded header from any other source is ignored. Canonical IP values and
normalized account identifiers are HMACed with separate purpose/version keys.
Raw values are neither persisted nor logged. Inactive bucket rows are deleted
48 hours after their last update.

All limits are cumulative: a request must pass every listed bucket. Rejection
uses the common `ApiError` with code `rate_limited`, HTTP `429`, an integer
`Retry-After`, and no account-existence detail. Limits never change the account
status and therefore cannot be used to permanently lock another user out.

Tokens refill continuously; “full refill” means the time to restore an empty
bucket to its stated capacity.

| Policy | Subject and capacity | Full refill / retry contract |
| --- | --- | --- |
| `register.marker` | anonymous marker: 5 | 15 min; additionally capacity 20 / full refill 24 h |
| `login.account` | normalized identifier: 5 | 15 min after failed login; successful login clears this bucket |
| `login.marker` | anonymous marker: 30 | 15 min; success does not clear marker protection |
| `verify.marker` | anonymous marker: 20 | 15 min |
| `resend.account` | normalized identifier: 3 | 60 min; same public response whether delivery occurs or not |
| `resend.marker` | anonymous marker: 10 | 60 min |
| `forgot.account` | normalized identifier: 3 | 60 min; same public response whether delivery occurs or not |
| `forgot.marker` | anonymous marker: 10 | 60 min |
| `reset.marker` | anonymous marker: 10 | 15 min |
| `refresh.family` | session family: 20 | 10 min; reuse detection is independent and always runs |
| `project.create` | actor user: 10 | 60 min |
| `project.mutate` | actor user: 60 | 60 min |
| `admin.users.mutate` | actor super-admin: 30 | 60 min |

The table stores integer capacity/refill configuration in application code, not
user-controlled rows. `Retry-After` is rounded up from the server clock. A
temporary PostgreSQL failure fails closed for login/reset/admin mutations and
returns `503`; it does not silently bypass limiting.

## Security audit contract

`audit_events` is append-only through the Audit module. HTTP handlers cannot
write arbitrary metadata. Each event type owns an allowlist; request bodies,
headers, cookies and free-form errors are rejected at the audit boundary.

Common fields are: event id, server time, event type, result/reason code,
request id, nullable actor user id, nullable subject user/project id and
nullable session family id. User IDs are retained as security/account history;
anonymous identifier/IP HMAC markers stay only in short-lived limiter rows and
are not copied into the 365-day audit stream.

| Event type | Result/reason allowlist | Allowed identifiers |
| --- | --- | --- |
| `account.registration_accepted` | `pending_verification` | subject user |
| `account.email_verified` | `success`, `invalid_or_expired` | subject user only on success |
| `auth.login` | `success`, `invalid_credentials`, `unverified`, `disabled`, `rate_limited` | actor/session family only on success |
| `auth.logout` | `success`, `already_revoked` | actor/session family when resolved |
| `auth.refresh` | `success`, `expired`, `revoked`, `reuse_detected`, `rate_limited` | actor/session family when resolved |
| `auth.password_reset_requested` | `accepted`, `rate_limited` | no account identifier |
| `auth.password_reset_completed` | `success`, `invalid_or_expired` | subject user only on success |
| `auth.csrf_rejected` | `origin`, `referer`, `fetch_metadata`, `token`, `content_type` | actor/session family when resolved |
| `account.disabled` / `account.restored` | `success`, `version_conflict`, `last_super_admin`, `forbidden` | actor and subject user |
| `project.created` / `project.updated` / `project.deleted` / `project.customer_assigned` | `success`, `forbidden`, `not_found`, `version_conflict`, `rate_limited` | actor and project; customer user only for successful assignment |

`invalid_credentials` deliberately covers absent login/email and wrong password.
It contains no submitted identifier. Operational logs may contain `request_id`,
HTTP status family, route template and internal error class, but never audit
metadata with user content. Security audit retention is 365 days, followed by a
bounded, observable deletion job; legal/incident hold is an explicit separate
state, not an infinite silent retention.

## Immediate revocation contract

Disable, password reset, security-secret compromise and global-role change run
in one database transaction:

1. lock/update the account with its optimistic `version`;
2. increment `security_version` and set the new account/role state;
3. revoke every active session and refresh family for that account;
4. append the allowlisted audit event;
5. commit, then clear cookies or enqueue external notifications.

Every protected request resolves the access hash and checks `users.status =
'active'`, session revocation/expiry and the session's captured
`security_version` against the current account. A request authenticated after
the revocation commit is denied even if its browser still has cookies. Restore
never restores old sessions; the user must log in again. A transaction already
authorized before the revocation commit may finish, and this residual race is
recorded as an R1 limitation; destructive admin writes recheck the actor version
inside their transaction before commit.

Refresh-token reuse revokes only the affected family unless the account is
disabled/compromised, in which case all families are revoked. Logout is
idempotent and revokes its family.

## Last active super-admin invariant

An account counts as an active super-admin only when `status='active'` and
`global_role='super_admin'`. Disable, demotion and deletion serialize on one
transaction-scoped PostgreSQL advisory lock owned by the Identity module. Under
that lock the service locks the target row, recounts active super-admins and
rejects the write with `409 last_super_admin` if the resulting count would be
zero. The same guard is used by every write path, including self-disable and
bootstrap cleanup.

This is not implemented as `SELECT count(*)` followed by an unlocked update.
Two administrators trying to disable each other concurrently cannot both
commit. Restore is idempotent but does not create a session. Database access
outside the Identity service is not allowed to modify `status/global_role`.

## Negative scenario contracts

| Scenario | HTTP/result | Data and audit invariant |
| --- | --- | --- |
| Absent account vs wrong password | identical `401 invalid_credentials` | no target user id or submitted identifier in audit/logs |
| Forgot/resend for absent, active or disabled account | identical `202` unless the caller itself is rate-limited | no account existence leak; mail queued only when policy allows |
| Limit exhausted | `429 rate_limited` + `Retry-After` | account remains active; bucket consume is concurrency-safe |
| Limiter unavailable on login/reset/admin mutation | `503 service_unavailable` | operation does not execute and no bypass occurs |
| Disabled account presents old access or refresh cookie | `401 unauthenticated` | no session recreated; revoked state unchanged |
| Replaced refresh token is replayed | `401 unauthenticated` | entire family revoked atomically; `reuse_detected` audit |
| Password reset races refresh | exactly one serializable outcome | successful reset leaves every pre-reset family revoked |
| Missing/wrong CSRF or cross-origin mutation | `403 csrf_rejected` | domain mutation is not entered; allowlisted rejection audit only |
| Non-admin calls users disable/restore | `403 forbidden` | target and sessions unchanged |
| Stale `If-Match` disables/restores | `409 conflict` | target and sessions unchanged |
| Last active super-admin is disabled/demoted | `409 last_super_admin` | target/version/sessions unchanged; rejection audit |
| Two active super-admins disable each other concurrently | one may commit; the second gets `409 last_super_admin` | at least one active super-admin remains |
| Restore disabled user | `200` on first and idempotent repeat | no old sessions revived; new login required |

## Verification gates

- Table-driven unit tests cover every policy boundary at `capacity-1`,
  `capacity`, refill edge and rounded `Retry-After` using an injected clock.
- PostgreSQL 17 integration tests run 20 concurrent bucket consumes and prove
  accepted requests never exceed capacity.
- Concurrent disable/demote tests use two independent transactions and prove the
  last-super-admin invariant.
- Revoke/reset/refresh race tests assert rows, versions and audit events after
  both commit orders and forced rollback.
- Contract tests compare public status, code and response shape for enumeration
  cases; timing is measured statistically in the load/security test, not claimed
  equal by unit assertions.
- Log-capture tests fail on password, token, cookie, login, email or raw IP
  sentinels. Audit metadata schema rejects unknown keys.
