# OpenAPI contracts

`contact-api.yaml` is the single source of truth for the public contact API.
`account-api.yaml` is the R1 source of truth for account, session, project and
super-admin user-management HTTP contracts. The two contracts deliberately stay
separate until the account handlers are implemented, so current contact codegen
does not create unimplemented server methods.

## Generation plan

After the application workspaces are initialized, generation runs in CI and locally from this file only:

| Target | Tool | Planned output | Rule |
| --- | --- | --- | --- |
| Go API | `oapi-codegen` | `apps/api/internal/api/generated/` | Generate request/response models and server interface; implementation stays outside generated files. |
| Next.js | `openapi-typescript` | `apps/web/src/shared/api/generated/` | Generate transport types; `src/shared/api/` owns the native `fetch` transport. |

R1 account generation uses a separate config/output package and is added with
the account implementation task. Until then CI/lint must parse
`account-api.yaml` independently and must not merge generated account methods
into the current contact server interface.

Run `npm run lint:openapi` for Redocly semantic linting of both contracts and
`npm run validate:openapi-adapter` for linting plus Go code generation/parsing.

Generated output is reproducible, committed only if the future workspace policy requires it, and never edited manually. CI validates the contract and fails on generated-code drift.

## Contract conventions

- `POST /v1/contact-submissions` is public and has no authentication scheme.
- Success returns `201` only after a submission is stored.
- Validation, cooldown and anti-abuse errors use the common `ApiError` envelope.
- The one-hour anonymous cooldown returns `429` and a required `Retry-After` header.
- Account endpoints use an opaque HttpOnly cookie plus a synchronizer CSRF
  header on every state-changing authenticated request.
- Account errors use one `ApiError` envelope with `code`, safe `message`,
  `requestId` and optional field messages.
- List endpoints are bounded and use opaque cursor pagination.
- A `404` for project/user resources may mean absent or hidden; responses do not
  expose the existence of another actor's object.
