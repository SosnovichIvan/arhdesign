# Public API and future operator authentication boundary

## Current public surface

`POST /api/v1/contact-submissions`, `/healthz` and `/readyz` are intentionally public. The OpenAPI operation declares `security: []`; no user account, access token, refresh token or operator route exists in the application today. The frontend must not place credentials in `localStorage`, `sessionStorage`, URLs, analytics events or request bodies.

The anonymous one-hour cooldown is not authentication. It uses an opaque random browser cookie and persists only an HMAC hash of that value for the limited anti-repeat purpose.

## Accepted R1 account boundary

The accepted R1 client-cabinet change uses an opaque server-side session rather
than a browser-readable JWT. Exact parameters and the test plan are owned by
`docs/architecture/client-cabinet-r1-auth-security-adr.md`; this table is only a
boundary summary.

| Item | Policy |
| --- | --- |
| Session identifier | At least 256 bits of cryptographically random data; only an HMAC hash is stored server-side. |
| Browser storage | HttpOnly `__Host-arhdesign_session` and path-scoped refresh cookies; readable synchronizer CSRF cookie. All use `Secure`, `SameSite=Strict`, no `Domain`; never local/session storage. |
| Lifetime | Access: 30-minute idle, 24-hour absolute. Refresh family: 30-day inactivity, 90-day absolute; rotate on every use and revoke family on reuse. |
| Transport | HTTPS only; protected API requests require the cookie and same-origin / CSRF verification. |
| Revocation | Server-side session is revoked on logout, credential reset, account disable, privilege change or suspected compromise. |

The current contact endpoint remains public. R1 protected routes are specified
separately in `openapi/account-api.yaml` until their handlers are implemented.

## Threat-model review

| Threat | Current control |
| --- | --- |
| Token theft from browser storage | No client-side auth tokens; future session is HttpOnly. |
| Cross-site form abuse | Origin-aware browser controls, honeypot, per-IP rate limit and anonymous cooldown. |
| Session fixation | Future session ID rotates at login and privilege changes. |
| CSRF against future operator mutations | Future protected mutations require same-origin and CSRF verification. |
| Credential leakage in logs | PII/logging policy prohibits secrets, cookies and request payloads in logs. |

R1 implementation must satisfy its accepted OpenSpec change, security ADR and
contract tests before protected routes are released.
