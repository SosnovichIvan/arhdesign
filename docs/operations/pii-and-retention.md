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

`BACKUP_LOCAL_RETENTION_DAYS` defaults to 7. The `postgres-backup` service creates a custom-format logical backup every 24 hours in the `postgres_backups` volume and removes expired local archives. Encrypted Restic snapshots in the independent S3 repository retain 30 daily and 12 monthly copies. Caddy writes JSON access logs to its protected data volume, rolls them at 10 MiB and retains them for at most 30 days. Docker container stdout/stderr logs rotate at 10 MiB with no more than five files per service.

The same-VPS backup is the current implementation, not the final disaster-recovery
boundary. The proposed encrypted off-site copy, restore drills and PITR rollout
are defined in [`postgresql-backup-and-recovery.md`](postgresql-backup-and-recovery.md).
This document must be updated again when that proposal is implemented so the
published retention and data-location statements match the actual storage.

Cooldown rows expire after one hour and are removed on access and by the daily cleanup. Production PostgreSQL and its backup volume must remain on the Russian VPS described in the policy.

## Chats and technical support data

Global chat membership, messages, read markers and timestamps are stored in
PostgreSQL on the same VPS. A global chat does not grant access to a project;
project chats remain a separate authorization scope. Telegram delivery is
queued only for a recipient who still has access and an active verified private
bot binding. Chat text is encrypted in the outbox and must not be copied into
application logs or technical incidents.

Authenticated users may send a complaint or suggestion from the cabinet. The
application stores the category, message, account ID, page path without query
parameters, timestamps and a pseudonymous fingerprint in `technical_reports`
for up to 90 days. The message is also delivered through the encrypted outbox
to the active subscribed `technical_admin` account in Telegram. The form warns
the user not to include passwords, payment credentials or other secrets.

Frontend runtime incidents and backend `5xx`/panic incidents contain only a
sanitized summary, route template/path without query parameters, HTTP method or
status where applicable, request ID and fingerprint. Request/response bodies,
cookies, authorization headers, tokens, raw URLs, chat text and stack traces are
not stored or sent. Repeated automatic incidents are aggregated in a bounded
window; expected validation, authentication and authorization `4xx` responses
are operational metrics and are not paged to Telegram.

## Logging and operations

Application logs must never contain form fields, cookies, cooldown tokens, HMAC secrets, SMTP credentials, Telegram bot tokens, database URLs or notification payloads. Structured events may include adapter name, HTTP status category, error class and deleted row count only.

### Telegram account confirmation

If a registrant chooses Telegram confirmation, the login and password entered
in the private bot chat pass through Telegram infrastructure. The application
uses the password only in memory for one credential comparison and must never
write it to PostgreSQL, cache, logs, traces, metrics, audit records, outbox rows
or error payloads. The bot requests deletion of the original password message
immediately after both successful and unsuccessful checks; a failed deletion
is retried within a bounded budget and raises an alert containing no chat text
or credentials.

Deleting a Telegram message does not prove erasure of every provider-side
technical copy. Before password input, the bot must disclose this external
transfer and offer the email-confirmation alternative. The published
personal-data documents must identify Telegram as a recipient/infrastructure
provider for this optional channel before the feature is released; this
operational rule alone is not a legal-compliance determination.

Access to PostgreSQL, backup volumes and VPS environment files is restricted to the site operator. Cloudflare Worker invocation logs must be disabled because relay requests contain notification text. Telegram notification copies are temporary and must not be used as a second archive.

## Verification

1. Confirm the production `.env` contains `CONTACT_RETENTION_DAYS=365`, `TELEGRAM_NOTIFICATION_RETENTION_HOURS=24`, `BACKUP_LOCAL_RETENTION_DAYS=7`, `BACKUP_REMOTE_DAILY_RETENTION=30` and `BACKUP_REMOTE_MONTHLY_RETENTION=12`.
2. Verify the published PDF hashes match the application constants before every release.
3. Check startup and cleanup logs only for aggregate events, never form content.
4. Confirm the latest PostgreSQL backup exists and that files older than the retention window disappear automatically.
5. Send a test request, confirm receipt in an authorised Telegram chat and verify its automatic deletion after the configured deadline.
6. Run repository and notification integration tests before changing migrations or cleanup logic.
7. Rotate `COOLDOWN_HMAC_SECRET`, Telegram token and relay secret through protected runtime secrets, never through Git.
8. Exercise Telegram confirmation with correct and incorrect credentials and
   verify that password text is absent from storage/logging and that
   `deleteMessage` is attempted in both outcomes.
9. Verify a synthetic frontend error and a backend `5xx` create one sanitized
   technical report and only the bound technical administrator receives it.
10. Submit a complaint from the cabinet and verify its 90-day retention marker,
    encrypted outbox payload and technical-admin-only recipient set.

## External legal actions (not automated)

The repository deliberately does not automate checking the operator in the Roskomnadzor register or filing a separate cross-border data-transfer notice. These two actions remain organisational TODOs for the operator.
