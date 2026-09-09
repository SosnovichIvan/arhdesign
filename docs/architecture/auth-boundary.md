# Public API and future operator authentication boundary

## Current public surface

`POST /api/v1/contact-submissions`, `/healthz` and `/readyz` are intentionally public. The OpenAPI operation declares `security: []`; no user account, access token, refresh token or operator route exists in the application today. The frontend must not place credentials in `localStorage`, `sessionStorage`, URLs, analytics events or request bodies.

The anonymous one-hour cooldown is not authentication. It uses an opaque random browser cookie and persists only an HMAC hash of that value for the limited anti-repeat purpose.

## Future operator boundary

When a protected operator UI is approved as a separate OpenSpec change, it will use an opaque server-side session rather than a browser-readable JWT:

| Item | Policy |
| --- | --- |
| Session identifier | At least 256 bits of cryptographically random data; only an HMAC hash is stored server-side. |
| Browser storage | `__Host-arhdesign_session` cookie only: `Secure`, `HttpOnly`, `SameSite=Strict`, `Path=/`, no `Domain`. Never local/session storage. |
| Lifetime | 8-hour idle lifetime, 24-hour absolute lifetime; rotate session ID on login and privilege changes. |
| Transport | HTTPS only; protected API requests require the cookie and same-origin / CSRF verification. |
| Revocation | Server-side session record is deleted on logout, credential reset, operator removal or suspected compromise. |

The reserved `operatorSession` OpenAPI security scheme documents this future boundary but is not applied to any existing public endpoint.

## Threat-model review

| Threat | Current control |
| --- | --- |
| Token theft from browser storage | No client-side auth tokens; future session is HttpOnly. |
| Cross-site form abuse | Origin-aware browser controls, honeypot, per-IP rate limit and anonymous cooldown. |
| Session fixation | Future session ID rotates at login and privilege changes. |
| CSRF against future operator mutations | Future protected mutations require same-origin and CSRF verification. |
| Credential leakage in logs | PII/logging policy prohibits secrets, cookies and request payloads in logs. |

Any introduction of login, protected endpoints or token issuance requires a new approved OpenSpec change, a security review and contract tests before implementation.
