## 1. Contract and persistence

- [ ] 1.1 Define webhook request boundary, secret-token validation and Telegram update fixtures; result: malformed/non-Telegram input, wrong secret and private-chat constraints have tests.
- [ ] 1.2 Add migration and repository for active/inactive Telegram subscribers; result: idempotent subscribe/unsubscribe and no password persistence verified against isolated PostgreSQL.

## 2. Bot subscription and notifications

- [ ] 2.1 Implement interactive «Подписаться»/«Отписаться» menu and login-then-password dialog with constant-time credential checks and credential-safe logging; result: table-driven tests cover valid, invalid, duplicate and non-private chat cases.
- [ ] 2.2 Fan out saved contact submissions to all active Telegram subscribers; result: one delivery failure does not prevent subsequent recipient delivery or persistence.
- [ ] 2.3 Add runtime env validation for Telegram settings and compose mapping; result: partial configuration fails fast, while an empty subscriber table remains valid.

## 3. Deferred release delivery (TODO — not part of current Apply)

- [ ] 3.1 Create tag-gated GitHub Actions deploy workflow; result: only `v*` tag whose commit belongs to `main` can deploy, and all runtime values are read from GitHub secrets.
- [ ] 3.2 Add safe remote `.env` materialization, exact-tag checkout, Compose migration/build/up, Telegram webhook setup and health/readiness checks; result: workflow does not print secrets and fails on remote health error.
- [ ] 3.3 Extend `.env.example`, root README and infra README; result: documentation states exactly where domain is set (`CADDY_SITE`, `NEXT_PUBLIC_SITE_URL`) and lists GitHub secrets without their values.

## 4. Verification and context

- [ ] 4.1 Run Go vet/race/coverage, frontend quality gates, generated-contract check and local Docker health; result: all gates pass without lowering coverage.
- [ ] 4.2 Update `.ai/context/`, validate it and conduct semantic review against code, database migration, workflow and README; result: `bash .ai/scripts/validate-context.sh` passes.
