# ADR: R1 password, token, session and CSRF policy

- Status: accepted for R1 implementation
- Date: 2026-09-22
- Scope: account registration, email/Telegram confirmation, login,
  refresh/logout and password recovery from `openapi/account-api.yaml`
- Owners: Identity and Session modules in the Go modular monolith

## Context and invariants

R1 uses a password as the only authentication factor. The browser must never
receive a bearer token that JavaScript can persist. PostgreSQL is the source of
truth for immediate session revocation. Raw passwords, session values,
verification/reset/refresh tokens and CSRF values are prohibited in logs,
analytics, URLs sent to the server, audit payloads and database columns.

Исключение, явно принятое владельцем продукта 2026-09-24: пользователь может
отправить password Telegram-боту для однократного подтверждения pending account.
Это не отменяет запрет на storage/logging и требует специальных мер ниже.

This ADR replaces the provisional future-session values in
`docs/architecture/auth-boundary.md`. Any weaker runtime value is a deployment
error, not an environment-specific option.

## Decision 1: password input and storage

| Item | R1 decision |
| --- | --- |
| Length | 15–128 Unicode code points for registration/reset. Login accepts 1–128 so invalid legacy input receives the same credential error. |
| Characters | Printable Unicode and spaces are accepted. Apply Unicode NFC before length check and hashing; never trim, case-fold or silently rewrite another part of the password. |
| Rules | No mandatory upper/lower/digit/symbol composition and no periodic forced change. |
| Blocklist | Reject the complete normalized value when it matches the versioned local compromised/common/context-specific password list. Include project/domain name and normalized login/email derivatives. Never send the password to an external breach/blocklist service; the owner-approved Telegram confirmation exception is governed separately by Decision 1a. |
| UX | Allow paste, password managers and a reveal control; explain length or blocklist rejection without echoing the password. |
| Algorithm | Argon2id PHC string, profile `argon2id-v1`: memory `19,456 KiB`, iterations `2`, parallelism `1`, random salt `16 bytes`, output `32 bytes`. |
| Calibration | Before production release, benchmark on the target VM under the configured auth concurrency. Increase cost to a measured 100–250 ms p95 when capacity permits; never go below `argon2id-v1`. Store algorithm/version/parameters with every hash. |
| Upgrade | After a successful comparison, rehash in the same request when the stored profile is weaker. A failed comparison never rewrites the credential. |

The baseline follows the current OWASP Argon2id minimum. The 15-character
minimum, ≥64-character support, blocklist, NFC and absence of composition rules
follow NIST SP 800-63B for a single-factor password verifier.

The bootstrap super-admin is not an alternate password path. On first valid
bootstrap, its secret is normalized, validated and hashed by this policy. The
plaintext environment value is no longer accepted after bootstrap completion.

### Decision 1a: Telegram confirmation по login/password

- Сценарий доступен только в private chat после перехода с post-registration
  экрана по ссылке на официального бота.
- Бот сначала запрашивает login, затем явно предупреждает, что password будет
  передан через инфраструктуру Telegram, и только после этого запрашивает его.
- Login хранится только в краткоживущем bot-auth state с TTL 5 минут. Password
  существует только в памяти обработки одного update и не сохраняется в БД,
  cache, log, trace, audit, metric, outbox или error.
- При success и failure бот немедленно вызывает `deleteMessage` для исходного
  password message. Ошибка удаления имеет bounded retry и технический alert без
  chat text/credentials.
- Проверка использует тот же Argon2id verifier и PostgreSQL rate limit, что web
  login. Ответ не уточняет, неверен login или password.
- Успех для `pending_verification` одной транзакцией активирует account,
  привязывает private `chat_id`, отзывает email verification tokens и пишет
  allowlisted audit event. Для `active` account тот же flow только связывает или
  подтверждает chat; `disabled` account отклоняется.
- Успешное Telegram-подтверждение не устанавливает `email_verified_at`.

Residual risk: приложение не контролирует технические копии Telegram и не может
доказать полное удаление message у внешнего провайдера. Риск принят владельцем;
он должен быть отражён в пользовательском предупреждении и документах о ПДн.

## Decision 2: opaque tokens

| Token | Raw value | Server storage | Lifetime and consumption |
| --- | --- | --- | --- |
| Email verification | 32 CSPRNG bytes, unpadded base64url | HMAC-SHA-256 with the verification purpose key | 24 hours; one successful atomic consume; resend revokes the previous active token |
| Password reset | 32 CSPRNG bytes, unpadded base64url | HMAC-SHA-256 with the reset purpose key | 30 minutes; one successful atomic consume; success replaces the credential and revokes all sessions |
| Access session | 32 CSPRNG bytes, unpadded base64url | HMAC-SHA-256 with the session purpose key | 30-minute sliding idle limit and 24-hour absolute limit |
| Refresh | 32 CSPRNG bytes, unpadded base64url | HMAC-SHA-256 with the refresh purpose key | single-use rotation; 30-day inactivity and 90-day family absolute limit |
| CSRF | 32 CSPRNG bytes, unpadded base64url | HMAC-SHA-256 or encrypted session-bound value | rotates on login, refresh and privilege change; dies with the session |

Purpose keys are derived from one runtime secret with explicit labels; they are
not reused as encryption, cooldown or Telegram secrets. Hash comparison is
constant-time. Each row stores `hash_key_version`; rotation keeps the immediately
previous key only through the longest outstanding token/session lifetime, then
retires it after all rows on that version are expired or revoked. Lookup plus
`consumed_at`/expiry update is one transaction with a row lock or equivalent
conditional update, so two consumers cannot both win.

Verification and reset links use a URL fragment, for example
`/reset-password#token=...`. The frontend reads the fragment, immediately
removes it with `history.replaceState` and sends the value only in the JSON POST
body. GET never consumes a token, which prevents mail scanners from applying
it. Auth pages set `Referrer-Policy: no-referrer` and do not load third-party
resources before the fragment is removed.

Forgot-password and resend always return the same `202` shape and comparable
work for existing, absent, pending, active or disabled accounts. Delivery is
queued only when allowed. The response never exposes account existence.

## Decision 3: session cookies and rotation

| Cookie | Attributes | Purpose |
| --- | --- | --- |
| `__Host-arhdesign_session` | `Secure; HttpOnly; SameSite=Strict; Path=/`; no `Domain`; short `Max-Age` matching access expiry | Opaque access-session value for protected requests |
| `__Secure-arhdesign_refresh` | `Secure; HttpOnly; SameSite=Strict; Path=/api/v1/auth`; no `Domain`; persistent only when `rememberMe=true` | Single-use refresh value available only to refresh/logout auth routes; `__Secure-` is used because a path-scoped cookie cannot satisfy the `__Host-` prefix rule |
| `__Host-arhdesign_csrf` | `Secure; SameSite=Strict; Path=/`; no `Domain`; not `HttpOnly` | Session-bound synchronizer value copied to `X-CSRF-Token` |

Local HTTP development may use different non-`__Host-` names and omit `Secure`
only in a documented development profile. Production startup fails unless the
public origin is HTTPS and all production attributes above are enabled.

Login creates a new family and new access/refresh/CSRF values. Refresh performs
one transaction: lock the presented token, reject expired/revoked/replaced
tokens, create the replacement, link `replaced_by_session_id`, and revoke the
old value. Reuse of a replaced refresh value revokes the entire family. Logout
is idempotent and revokes the family. Password reset, account disable, suspected
compromise and global-role change revoke all applicable sessions immediately.

`rememberMe=false` makes the refresh cookie non-persistent; it does not weaken
server expiry. `rememberMe=true` sets `Max-Age` no longer than the 30-day
inactivity limit. Access is rechecked against current account status and
security version on every protected request, not only when the cookie is issued.

## Decision 4: CSRF and browser boundary

All browser POST/PUT/PATCH/DELETE requests require `Content-Type:
application/json`. Authenticated mutations, including refresh/logout, require:

1. exact `Origin` match against the configured public origin; if `Origin` is
   absent, an exact-origin `Referer` is required; if both are absent, reject;
2. `Sec-Fetch-Site` of `same-origin` when the header is present;
3. `X-CSRF-Token` equal in constant time to both the readable CSRF cookie and
   the server-side session value.

Public auth mutations (register, verify, login, forgot and reset) require the
same Origin/Referer and Fetch Metadata checks but not a pre-existing CSRF token.
CORS is disabled by default; if enabled for a deployment, it is an exact allow
list, never `*` with credentials. `SameSite=Strict` is defense in depth and does
not replace these checks.

The API returns the common safe `ApiError` envelope. Secret mismatch details,
hash parameters, whether an account exists, raw database errors and token state
are never returned.

## Transaction boundaries

- Registration: user, profile, credential, verification token, audit and email
  outbox commit atomically; SMTP happens after commit.
- Verification: token consume, account activation and audit commit atomically.
- Telegram confirmation: Argon2id comparison happens before the transaction;
  account activation, private-chat binding, revocation of outstanding email
  verification tokens and the allowlisted audit event commit atomically.
  `deleteMessage` for the password message is attempted outside the transaction
  on every success and failure path so a Telegram outage cannot hold database
  locks or change the authentication result.
- Refresh: old-token consume/replacement or family revoke commits atomically.
- Reset: token consume, credential replacement, all-session revoke, audit and
  notification outbox commit atomically.
- Disable: account status/version, all-session revoke and audit commit atomically.

No database transaction remains open during Argon2 work or an SMTP/network
call. Retryable writes have an idempotency/domain uniqueness guard.

## Required test plan

| Layer | Required cases |
| --- | --- |
| Password unit | 14/15/128/129 code points; spaces and Unicode NFC equivalence; no composition rule; local blocklist; parameter encoding; correct/wrong compare; weaker-profile rehash; malformed PHC rejection without panic. |
| Token unit | CSPRNG length/encoding; purpose separation; no raw storage; correct/wrong/expired/consumed token; resend invalidation; constant-time comparison path. |
| Session service | login rotation; 30-minute idle and 24-hour access absolute expiry; refresh 30/90-day boundaries; concurrent refresh has one winner; replaced-token reuse revokes family; logout repeat; reset/disable/role-change revoke. |
| CSRF/HTTP | missing/wrong Origin; deceptive suffix origin; Referer fallback; both absent; cross-site Fetch Metadata; missing/mismatched header/cookie/session token; wrong content type; safe same-origin success. |
| Enumeration | login, forgot and resend return indistinguishable public shapes for absent/existing/disabled/unverified accounts; logs and audit contain no submitted password/token. |
| Telegram confirmation | private chat only; pending/active/disabled/absent accounts; correct/wrong credentials; bot-state expiry; shared rate limit; activation/binding race; password absent from database/cache/log/trace/audit/metric/outbox; `deleteMessage` attempted after success and failure, with bounded retry on API error. |
| PostgreSQL integration | token conditional consume race; refresh rotation race and rollback; session revoke transaction; unique token hashes; timestamps evaluated by the database clock policy. |
| E2E | register → fragment verify → login; forgot/reset with expired and reused link; remember-me cookie persistence; logout; disabled active session; browser back/referrer does not disclose a token. |
| Operational | production refuses HTTP/insecure cookie config; purpose-secret absence fails readiness; Argon2 benchmark and bounded auth concurrency fit the VM memory budget. |

Implementation cannot close R1-BE-001—003 until these cases exist at the
project coverage threshold and the PostgreSQL race cases run on PostgreSQL 17.

## Consequences and rejected options

- Stateful opaque sessions add PostgreSQL reads and cleanup work, but allow
  immediate disable and refresh-reuse detection.
- Strict cookies can require a second same-origin request after arriving from
  an external link; auth email flows are public and therefore remain usable.
- JWT in local/session storage is rejected because browser-readable bearer
  tokens increase XSS impact and make immediate revoke harder.
- Plain SHA-256 token storage is rejected; keyed, purpose-separated hashing
  limits offline verification after a database-only disclosure.
- Password composition and scheduled rotation are rejected in favor of length,
  a blocklist, throttling and breach-triggered reset.
- Telegram password entry has greater exposure than an application-controlled
  one-time confirmation token because Telegram receives the message. It is kept
  only as an explicit product requirement with a warning before input,
  immediate best-effort message deletion and a documented provider-side
  residual risk; application-side deletion cannot prove erasure of every
  Telegram technical copy.

## References

- OWASP Password Storage Cheat Sheet: <https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html>
- OWASP Session Management Cheat Sheet: <https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html>
- OWASP CSRF Prevention Cheat Sheet: <https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html>
- NIST SP 800-63B: <https://pages.nist.gov/800-63-4/sp800-63b.html>
