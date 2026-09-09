## Architecture

```text
Trusted operator → Telegram Bot webhook → Go API → PostgreSQL telegram_subscribers
Contact form → Go API → PostgreSQL submission → Telegram Bot API → active subscribers
Git tag from main → GitHub Actions → SSH to VPS → protected .env → Docker Compose
```

### Subscription flow

1. Telegram sends an update only to `POST /api/telegram/webhook`.
2. API verifies `X-Telegram-Bot-Api-Secret-Token` against `TELEGRAM_WEBHOOK_SECRET` with constant-time comparison; updates without it receive `401`.
3. The trusted operator chooses «Подписаться» from the reply keyboard (or sends `/subscribe`). The bot requests login, then password in separate messages and compares both values against `TELEGRAM_ADMIN_USERNAME` and `TELEGRAM_ADMIN_PASSWORD` in constant time. This is one system credential pair for the main administrator, not a visitor or subscriber account.
4. On success API upserts `chat_id`, `username`, timestamps and active status. It replies with a confirmation. No plaintext password, token, message body containing credentials, phone/email or project description is written to application logs.
5. Invalid credentials return a generic denial, trigger a rate limit keyed by chat ID, and never create a subscription.
6. `/unsubscribe` disables the current chat. Only «Подписаться» appears while inactive and only «Отписаться» while active.

The command contains a password in Telegram transit/history. This is acceptable only for a trusted operator account and a dedicated bot password that is not reused elsewhere. The bot deletes the credential-bearing command after processing when Telegram permissions allow it. A future stronger alternative is a single-use, short-lived subscription link; it is not introduced here because no operator web authentication exists.

### Notification delivery

After the contact submission transaction completes, the notifier reads active subscriber IDs and sends a formatted message to each one. A failed chat delivery is isolated and logged only with adapter name and a non-PII error class; it does not roll back the contact submission or stop delivery to other chats. An empty subscriber list is valid and results in no Telegram attempt.

### Contract and data

The public contact API remains anonymous and unchanged. The webhook is intentionally not exposed in the public OpenAPI browser contract; it has a dedicated request schema/test fixture because its caller is Telegram, not a portfolio visitor. The database stores only fields needed for delivery: signed `chat_id`, optional Telegram username, `subscribed_at`, `unsubscribed_at`, and timestamps.

### Secrets and deployment

No secret appears in Git, an image layer, logs or browser code. GitHub repository/environment secrets supply runtime `.env` fields to the deploy workflow. Required secrets:

| Secret | Purpose |
| --- | --- |
| `VPS_HOST`, `VPS_PORT`, `VPS_USER`, `VPS_SSH_KEY`, `VPS_DEPLOY_PATH` | SSH deployment target |
| `CADDY_SITE`, `NEXT_PUBLIC_SITE_URL` | public domain configuration |
| `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `COOLDOWN_HMAC_SECRET` | database and contact cooldown |
| `TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET` | Bot API and webhook authentication |
| `TELEGRAM_ADMIN_USERNAME`, `TELEGRAM_ADMIN_PASSWORD` | системные credentials главного администратора |

The deploy workflow triggers on `v*` tag pushes, rejects a tag whose commit is not an ancestor of `origin/main`, writes `.env` with mode `600` on the VPS, checks out the exact tag, runs Compose migration/build/up, configures the Telegram webhook with the final HTTPS domain, and verifies `/healthz` and `/readyz`. All runtime configuration changes are delivered only through GitHub secrets; local `.env` remains solely a development convention.

### Alternatives

| Option | Decision |
| --- | --- |
| Fixed `TELEGRAM_CHAT_ID` | Rejected: requires manual server change for every recipient and cannot unsubscribe. |
| Web login form | Rejected: creates a public auth surface and needs Figma, cookies, CSRF and MFA. |
| Telegram `/subscribe login password` | Chosen: minimal trusted-operator flow; credentials are env-only and not stored. |
| Deploy on every main push | Rejected: user requires a deliberate release tag. |
