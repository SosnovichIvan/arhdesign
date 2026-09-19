# Personal-data processing and retention operations

## Data handled by the contact form

The API stores the name, phone number or email address, project type, optional project description and submission timestamp in `contact_submissions`. Each accepted request creates one linked `contact_consents` row with the granted flag, method, exact checkbox text, consent version, published PDF path and SHA-256, source page URL and server timestamp. The anti-repeat cooldown stores only an HMAC hash of an anonymous cookie token; the raw token is never persisted in PostgreSQL.

The published documents are:

- [personal-data consent v4](../../public/documents/personal-data-consent-2026-09-19-v4.pdf);
- [personal-data policy v1](../../public/documents/personal-data-policy-2026-09-19-v1.pdf);
- the public HTML policy at `/privacy`.

The consent version, path and SHA-256 are constants in the frontend and API. A deployment must keep those constants and the published file in sync.

## Retention

`CONTACT_RETENTION_DAYS` defines the maximum age of contact submissions. It defaults to 365 days and accepts values from 1 through 3650. The API deletes older submissions at startup and every 24 hours; linked consent records are removed by foreign-key cascade.

`TELEGRAM_NOTIFICATION_RETENTION_HOURS` defaults to 24 and accepts values from 1 through 24. Every successful subscriber notification is registered in `telegram_notification_receipts`; the cleanup worker calls Telegram `deleteMessage` when the deadline is due and records the deletion time. If the receipt cannot be persisted after sending, the API immediately attempts a best-effort deletion.

`BACKUP_RETENTION_DAYS` defaults to 30. The `postgres-backup` service creates a compressed logical backup every 24 hours in the `postgres_backups` volume and removes expired archives. Caddy writes JSON access logs to its protected data volume, rolls them at 10 MiB and retains them for at most 30 days. Docker container stdout/stderr logs rotate at 10 MiB with no more than five files per service.

Cooldown rows expire after one hour and are removed on access and by the daily cleanup. Production PostgreSQL and its backup volume must remain on the Russian VPS described in the policy.

## Logging and operations

Application logs must never contain form fields, cookies, cooldown tokens, HMAC secrets, SMTP credentials, Telegram bot tokens, database URLs or notification payloads. Structured events may include adapter name, HTTP status category, error class and deleted row count only.

Access to PostgreSQL, backup volumes and VPS environment files is restricted to the site operator. Cloudflare Worker invocation logs must be disabled because relay requests contain notification text. Telegram notification copies are temporary and must not be used as a second archive.

## Verification

1. Confirm the production `.env` contains `CONTACT_RETENTION_DAYS=365`, `TELEGRAM_NOTIFICATION_RETENTION_HOURS=24` and `BACKUP_RETENTION_DAYS=30`.
2. Verify the published PDF hashes match the application constants before every release.
3. Check startup and cleanup logs only for aggregate events, never form content.
4. Confirm the latest PostgreSQL backup exists and that files older than the retention window disappear automatically.
5. Send a test request, confirm receipt in an authorised Telegram chat and verify its automatic deletion after the configured deadline.
6. Run repository and notification integration tests before changing migrations or cleanup logic.
7. Rotate `COOLDOWN_HMAC_SECRET`, Telegram token and relay secret through protected runtime secrets, never through Git.

## External legal actions (not automated)

The repository deliberately does not automate checking the operator in the Roskomnadzor register or filing a separate cross-border data-transfer notice. These two actions remain organisational TODOs for the operator.
