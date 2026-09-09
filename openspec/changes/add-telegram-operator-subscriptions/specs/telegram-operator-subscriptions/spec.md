## ADDED Requirements

### Requirement: Trusted Telegram subscription

The API MUST subscribe only a private Telegram chat whose interactive login/password flow matches the configured system credentials of the main administrator. The credentials MUST be read only from runtime environment/GitHub Secrets and MUST NOT be persisted or logged.

#### Scenario: Valid private-chat subscription

- **WHEN** Telegram sends an authenticated webhook update from a private chat with valid configured login and password
- **THEN** API upserts that chat as active, returns a confirmation through the bot, and stores no supplied password.

#### Scenario: Invalid or non-private subscription

- **WHEN** a webhook has invalid credentials, an invalid webhook secret, or a group/channel chat
- **THEN** API creates no active subscription, returns only a generic denial where applicable, and records no credential value in logs.

### Requirement: Subscriber-controlled delivery

The API MUST send every successfully persisted contact submission to every active Telegram subscriber and MUST allow an active private chat to unsubscribe.

#### Scenario: Multiple active recipients

- **WHEN** a valid contact form is saved and multiple active subscribers exist
- **THEN** API attempts delivery to each subscriber, while a failure for one recipient does not roll back the submission or block another recipient.

#### Scenario: Unsubscribe

- **WHEN** an active private chat sends `/unsubscribe`
- **THEN** API disables that chat and excludes it from later contact notifications.

### Requirement: Secret-only tag deployment

Runtime deployment configuration MUST originate from GitHub Actions secrets and deploy only a release tag that belongs to `main`.

#### Scenario: Eligible release tag

- **WHEN** a `v*` tag is pushed and its commit is reachable from `origin/main`
- **THEN** the workflow writes protected runtime configuration on the VPS, deploys that exact tag, configures the HTTPS Telegram webhook and verifies health endpoints without printing secret values.

#### Scenario: Ineligible tag

- **WHEN** a tag is not a `v*` release tag or its commit is not reachable from `origin/main`
- **THEN** no deployment runs.
