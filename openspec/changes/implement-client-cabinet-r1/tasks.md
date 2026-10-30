## 1. Contract and design

- [x] 1.1 `[R1-SPEC-001]` Оформить accepted OpenSpec proposal, design, capability specs и трассировку на R1 checklist; результат: change валиден и scope не включает R2–R7.
- [x] 1.2 `[R1-DES-001]` Подтвердить Figma routes auth/projects/users и внести стабильные Node ID; результат: все маршруты R1 связаны с frames.
- [x] 1.3 `[R1-DES-002]` Завершить states matrix; результат: default/loading/empty/success/validation/server/expired/forbidden/disabled покрыты.
- [x] 1.4 `[R1-DES-003]` Завершить 1440/768/390, Light/Dark, keyboard/focus/long-content; результат: design review без blocker.
- [x] 1.5 `[R1-DOC-001]` Обновить карту маршрутов, ролей, session/email и no-customer flow; результат: документация совпадает с specs.
- [x] 1.6 `[R1-SPEC-002]` Согласовать email/Telegram confirmation и recovery boundary; результат: Telegram проверяет registration login/password, атомарно активирует pending account и связывает private chat без дополнительной ссылки.
- [x] 1.7 `[R1-DES-004]` Обновить Figma channel-choice/bot-auth/recovery states; результат: desktop/tablet/mobile и input/error/rate-limit/success/switch-channel состояния полны. Evidence: section `692:10`, viewport groups `692:11`—`692:13`, visual/structural review 2026-09-24.
- [x] 1.8 `[R1-DOC-002]` Согласовать UX-тексты и recovery fallback; результат: тексты, TTL и предупреждения однозначны. Evidence: owner approval 2026-09-24; состояния отрисованы в R1-DES-004.

## 2. Data and API

- [x] 2.1 `[R1-DATA-001]` Согласовать Figma/Mermaid ERD R1; результат: entities и cardinalities полны.
- [x] 2.2 `[R1-DATA-002]` Заполнить data dictionary; результат: owner/nullability/uniqueness/delete/retention/PII/volume указаны для каждой таблицы.
- [x] 2.3 `[R1-DATA-003]` Описать query/index catalog; результат: каждый индекс связан с bounded query и планом.
- [x] 2.4 `[R1-API-001]` Добавить OpenAPI auth/projects/users/error envelope; результат: lint/generation проходит.
- [x] 2.5 `[R1-API-002]` Зафиксировать password/token/session/CSRF policy; результат: security ADR и tests plan согласованы.
- [x] 2.6 `[R1-API-003]` Зафиксировать limits/audit/revoke/last-super-admin; результат: отрицательные сценарии имеют contracts.
- [x] 2.7 `[R1-API-004]` Зафиксировать daily summary/alerts contract; результат: период, агрегаты и PII exclusions однозначны.
- [x] 2.8 `[R1-DATA-004]` Реализовать additive migration; результат: empty DB и upgrade current→R1 проходят на PostgreSQL 17.
- [x] 2.9 `[R1-DATA-005]` Реализовать auth audit/report ledger indexes/constraints; результат: duplicate delivery невозможна при гонке.
- [x] 2.10 `[R1-DATA-006]` Добавить Telegram bot-auth state/binding/preference migration; результат: no password storage, TTL, uniqueness, revoke и PostgreSQL 17 upgrade проверены.
- [x] 2.11 `[R1-API-005]` Добавить channel/bot-auth/status/webhook contracts; результат: generic responses и generated adapter проходят lint/drift check.

## 3. Backend and frontend

- [x] 3.1 `[R1-BE-001]` Реализовать registration/verify/resend и email outbox; результат: success/error/retry tests проходят.
- [x] 3.2 `[R1-BE-002]` Реализовать login/logout/refresh/bootstrap super-admin; результат: rotation/reuse/revoke tests проходят.
- [x] 3.3 `[R1-BE-003]` Реализовать forgot/reset; результат: expired/reused token и session revoke проверены.
- [x] 3.4 `[R1-BE-004]` Реализовать транзакционное project creation/late customer assignment; результат: no fake customer и unique active customer.
- [x] 3.5 `[R1-BE-005]` Реализовать super-admin users list/disable/restore; результат: last-super-admin invariant и access revoke проверены.
- [x] 3.6 `[R1-BE-006]` Реализовать metrics scheduler, daily summary, alerts и Telegram outbox; результат: restart/retry не создаёт дубль.
- [x] 3.7 `[R1-FE-001]` Реализовать auth UI; результат: design states, validation и safe redirect покрыты tests.
- [x] 3.8 `[R1-FE-002]` Реализовать protected layout/projects/create; результат: customer/no-customer paths работают по роли.
- [x] 3.9 `[R1-FE-003]` Реализовать super-admin users UI; результат: disable/restore states и confirmation доступны с клавиатуры.
- [x] 3.10 `[R1-BE-007]` Реализовать Telegram confirmation/reset delivery; результат: private credential verification, atomic activation/binding, password-message deletion, eligibility и retry проверены.
- [x] 3.11 `[R1-FE-004]` Реализовать выбор канала и Telegram handoff; результат: resend/switch/expired/unverified-email states соответствуют дизайну.

## 4. Automated verification

- [x] 4.1 `[R1-TEST-001]` Добавить frontend unit/component coverage; результат: authored code ≥90% lines/branches.
- [x] 4.2 `[R1-TEST-002]` Добавить Go domain/service authorization/security tests; результат: Go statement coverage ≥90% и все decision branches reviewed.
- [x] 4.3 `[R1-TEST-003]` Добавить PostgreSQL migration/concurrency tests; результат: target engine checks проходят.
- [x] 4.4 `[R1-TEST-004]` Добавить monitoring/auth aggregate/idempotency tests; результат: timezone, retry и PII exclusions проверены.
- [x] 4.5 `[R1-E2E-001]` Подключить Cypress/Mailpit/CI; результат: `npm run test:ui` стабильно запускается локально и в CI.
- [x] 4.6 `[R1-E2E-002]` Cypress registration/verify/login cases; результат: happy/duplicate/weak/unverified/resend проходят.
- [x] 4.7 `[R1-E2E-003]` Cypress reset/session/disable cases; результат: expired/reuse/logout/restore/revoke проходят.
- [x] 4.8 `[R1-E2E-004]` Cypress projects/no-customer/foreign URL cases; результат: happy paths и backend denial проходят.
- [x] 4.9 `[R1-TEST-005]` Добавить backend/DB tests Telegram auth delivery; результат: credentials/TTL/race/private-chat/deleteMessage/enumeration/log-redaction покрыты.
- [x] 4.10 `[R1-E2E-005]` Добавить Cypress mocked-Telegram flow; результат: register/confirm/reset и negative cases проходят.

## 5. Operations and release

- [x] 5.1 `[R1-QA-001]` Выполнить visual/accessibility review; результат: 1440/768/390 × themes, keyboard, zoom, axe, reduced motion без blocker.
- [x] 5.2 `[R1-OPS-001]` Включить encrypted off-site backup и restore drill; результат: measured RPO≤24h/RTO≤4h.
- [x] 5.3 `[R1-OPS-002]` Обновить env/secrets/migration/rollback/security runbooks; результат: deploy не требует недокументированного шага.
- [x] 5.4 `[R1-OPS-003]` Подготовить VM/limits/monitoring/load test; результат: capacity thresholds подтверждены или размер скорректирован.
- [x] 5.5 `[R1-OPS-004]` Настроить 15-minute metrics, daily Telegram summary и critical alerts; результат: alert drill и exactly-once daily delivery проходят.
- [x] 5.6 `[R1-OPS-005]` Документировать Telegram auth delivery и fallback; результат: bot username, secrets, diagnostics и manual recovery описаны.
- [x] 5.7 `[R1-QA-002]` Выполнить полный quality gate и Docker smoke; результат: frontend/backend/DB/Cypress/Playwright/build зелёные.
- [ ] 5.8 `[R1-REL-001]` Доставить через PR develop→main и tag `0.2.0`; результат: production health/monitoring/rollback проверены.
