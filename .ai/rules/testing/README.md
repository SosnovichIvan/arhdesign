# Testing and verification

- Каждый OpenSpec task содержит способ проверки и отмечается после него.
- Frontend: Vitest V8 coverage для authored `src/` MUST быть не менее 90% по lines, functions, branches и statements. Generated OpenAPI code, framework config, type declarations и test fixtures не входят в расчёт. Typecheck, unit/component tests и Playwright для критических form-flow обязательны; UI сверяется с Figma в Desktop/Tablet/Mobile и Light/Dark.
- Go: table-driven unit tests, `go test -race -count=1 ./...`, `go vet ./...`; atomic coverage profile для authored backend code MUST показывать не менее 90% statement coverage. Go standard tooling не считает branches/functions отдельно: каждый decision branch явно покрывается table-driven cases и проверяется на review. Generated code, migrations и test fixtures исключаются. Integration tests для PostgreSQL и внешних adapters использовать controllable fakes/test containers.
- E2E, visual, accessibility и contract tests не заменяют unit/component coverage. Изменение не может снижать достигнутое покрытие; исключение допустимо только с явным OpenSpec-обоснованием и review approval.
- Тесты не зависят от реального SMTP, Telegram или production VDS.
