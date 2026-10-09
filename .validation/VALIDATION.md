# Validator context

This document was generated after user approval. Validators consume `project-context.json`, `rules.json`, and `config.json`; this Markdown file explains that configuration.

## Detected project

- Stacks: nodejs
- Frontend roots: src
- Backend roots: apps/api
- Documentation: README.md, docs
- Test configurations: playwright.config.ts, vitest.config.ts

## Approved validators

test, frontend, backend

## Coverage policy

Lines: 90%; branches: 90%; changed lines: 90%.

## Backend inputs

- HTTP contract: openapi/contact-api.yaml
- Database engine: postgresql

## Test and frontend rules

- Test suffixes: .test.ts, .test.tsx, .spec.ts, .spec.tsx, _test.go
- Forbidden frontend tokens: eval(, dangerouslySetInnerHTML

## Limitations

A named HTTP contract or database engine records the user's decision; the corresponding parser must still be installed before those checks become verified. Re-run `agent-skills context` after changing project structure or rules.

## Project adapters

The bundled CLI remains a deterministic pre-check. Executable adapters for this
repository are declared in `adapters.json` and run through
`../scripts/validate-adapters.sh`:

- `frontend-vitest-v8` runs ESLint, TypeScript and the Vitest V8 90% threshold;
- `openapi-go-codegen` lints both HTTP contracts, regenerates contact bindings
  with `oapi-codegen` 2.5.0, rejects generated-code drift and parses the R1
  account contract into temporary Go types;
- `postgresql-migrations` uses `TEST_DATABASE_URL` for the isolated PostgreSQL
  integration test, which applies every migration;
- `go-api-quality` runs `go vet`, race tests, PostgreSQL checks, Go 90%
  coverage and the OpenAPI adapter.

Use `npm run validate:adapters` for both application contours. It requires an
isolated `TEST_DATABASE_URL` whose database name contains `test`.
