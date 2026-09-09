## 0. Governance and design gate

- [x] 0.1 Провести context-impact и final review Concept A в Figma; проверка: `.ai/context/` и design evidence для Desktop/Tablet/Mobile × Light/Dark. Design согласован пользователем.
- [x] 0.2 Утвердить Figma implementation node/version и contact-flow; проверка: явное user approval в change. Concept A и форма согласованы пользователем.
- [x] 0.3 Создать OpenAPI contract для submission API и error responses; проверка: OpenAPI validation и generation plan.

## 1. Frontend foundation

- [x] 1.1 Инициализировать Next.js App Router + TypeScript strict, FSD, утверждённые зависимости (Tailwind, React Hook Form, Zod, clsx, tailwind-merge, class-variance-authority) и lint/typecheck/test gates; проверка: clean build and typecheck, dependency audit без Axios/lodash/TanStack Query.
- [x] 1.2 Реализовать semantic Light/Dark tokens, Tailwind theme, shared UI primitives и reusable product components; проверка: visual comparison с Figma, no CSS Modules/styled-components/local duplicates, reuse audit для `shared/components`.
- [x] 1.3 Реализовать routes: landing, catalog and three project pages; проверка: Desktop/Tablet/Mobile × Light/Dark без overflow.
- [x] 1.4 Реализовать Hero и Project Carousel из утверждённых Figma правил; проверка: keyboard navigation, controls, loop and responsive visual review.
- [x] 1.5 Реализовать contact overlay и submit states (idle, loading, success, failure, cooldown); проверка: Playwright flow, `Retry-After` countdown и accessibility review.
- [x] 1.6 Добавить VK и Telegram иконки/ссылки во всех утверждённых местах; проверка: ссылки открывают соответственно `https://vk.ru/studio_architecture_design` и `https://t.me/studio_architecture_design` в новой безопасной вкладке.
- [x] 1.7 Добавить SEO: metadata, canonical, robots, sitemap, JSON-LD, OG and image alt text; проверка: server-rendered HTML audit.
- [x] 1.8 Настроить frontend coverage gate: не менее 90% lines/functions/branches/statements для authored `src/`; проверка: Vitest coverage report и CI threshold failure ниже лимита.

## 2. Go API and data

- [x] 2.1 Создать Go API module, config, structured logging, health/readiness and graceful shutdown; проверка: `go vet` and health endpoints.
- [x] 2.2 Реализовать OpenAPI-generated submission endpoint, server validation, one-hour anonymous cooldown, rate limit and honeypot; проверка: handler/service table-driven tests для повторной заявки, `429` и `Retry-After`.
- [x] 2.3 Добавить PostgreSQL migrations and repository for submissions; проверка: integration test with isolated database.
- [x] 2.4 Реализовать email and Telegram notification adapters with timeout/error isolation; проверка: adapter tests with fakes, no external network.
- [x] 2.5 Добавить retention/PII logging policy и operator documentation; проверка: security review and configuration audit.
- [x] 2.6 Зафиксировать future operator-auth boundary: отсутствие auth/token у public endpoints и cookie/token policy для защищённых endpoint; проверка: OpenAPI security-scheme review, threat-model review и отсутствие токенов в frontend storage.
- [x] 2.7 Настроить Go coverage gate: не менее 90% total statement coverage для authored backend code и table-driven branch cases; проверка: atomic coverage profile и threshold script в CI. Результат: 90.1% с изолированной PostgreSQL test-базой.

## 3. VDS and delivery

- [x] 3.1 Подготовить Dockerfiles and Docker Compose for web, api, postgres and Caddy; проверка: local compose start and healthchecks.
- [x] 3.2 Настроить reverse proxy, TLS-ready domains, internal service network and env examples; проверка: no secrets in images/repository and only proxy public ports.
- [x] 3.3 Описать VDS deploy, migrations, backup/restore and rollback; проверка: documented dry-run.
- [ ] 3.4 Настроить CI quality gates: OpenSpec, frontend, Go, generated contract drift, 90% coverage и security scan; проверка: pipeline run.

## 4. Acceptance

- [x] 4.1 Провести UX/design review с Figma для всей матрицы экранов; проверка: evidence recorded in `docs/reviews/final-semantic-review.md`, no blocking deviations.
- [x] 4.2 Прогнать unit, integration, e2e, axe and API contract tests; проверка: green reports.
- [ ] 4.3 Провести pre-production review без внешнего deploy; проверка: user approval for VDS deployment requested separately.
- [x] 4.4 Обновить `.ai/context/`, документацию и выполнить `bash .ai/scripts/validate-context.sh`; проверка: semantic context review in `docs/reviews/final-semantic-review.md`.
