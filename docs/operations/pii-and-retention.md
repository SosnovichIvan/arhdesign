# Personal-data and retention policy

## Data handled by the contact form

The API stores only the name, selected contact method, project type, project description and the submission timestamp. The anti-repeat cooldown stores only an HMAC hash of an anonymous cookie token; the raw token is never persisted in PostgreSQL.

## Retention

`CONTACT_RETENTION_DAYS` defines the maximum age of contact submissions. It defaults to 365 days and accepts values from 1 through 3650. The API deletes older submissions at startup and every 24 hours. The operator must set the value in the VDS runtime `.env` according to the approved privacy notice and applicable requirements; changing it affects future cleanup runs.

Cooldown rows expire after one hour and are removed when they are read after expiry. Backups follow the same access and retention controls as the primary database.

## Logging and operations

Application logs must never contain form fields, cookies, cooldown tokens, HMAC secrets, SMTP credentials, Telegram bot tokens, database URLs or notification payloads. Structured events may include adapter name, HTTP status category, error class and deleted row count only.

Access to PostgreSQL backups and VDS environment files is restricted to the site operator. Before production launch, publish a privacy notice that states the purpose of collecting requests, the chosen retention period and the contact for data-subject requests.

## Verification

1. Confirm `CONTACT_RETENTION_DAYS` is set in the server `.env` and is consistent with the privacy notice.
2. Check startup logs only for aggregate cleanup events, never form content.
3. Run the isolated repository integration test before changing migrations or cleanup logic.
4. Rotate `COOLDOWN_HMAC_SECRET`, SMTP credentials and Telegram token through the VDS environment, never through Git.
