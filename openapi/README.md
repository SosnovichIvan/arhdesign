# OpenAPI contracts

`contact-api.yaml` is the single source of truth for the public contact API.

## Generation plan

After the application workspaces are initialized, generation runs in CI and locally from this file only:

| Target | Tool | Planned output | Rule |
| --- | --- | --- | --- |
| Go API | `oapi-codegen` | `apps/api/internal/api/generated/` | Generate request/response models and server interface; implementation stays outside generated files. |
| Next.js | `openapi-typescript` | `apps/web/src/shared/api/generated/` | Generate transport types; `src/shared/api/` owns the native `fetch` transport. |

Generated output is reproducible, committed only if the future workspace policy requires it, and never edited manually. CI validates the contract and fails on generated-code drift.

## Contract conventions

- `POST /v1/contact-submissions` is public and has no authentication scheme.
- Success returns `201` only after a submission is stored.
- Validation, cooldown and anti-abuse errors use the common `ApiError` envelope.
- The one-hour anonymous cooldown returns `429` and a required `Retry-After` header.
