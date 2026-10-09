# R1 release readiness

Дата проверки: 2026-10-09. Область: R1 личного кабинета, аутентификация,
проекты, календарь, чаты, документы, уведомления, мониторинг и локальный Docker.

## Итог

Quality gate `R1-QA-002` пройден. Блокирующих дефектов по проверенной области
не осталось. Локальный стенд пересобран из текущего рабочего дерева и доступен
по `http://localhost/`.

| Проверка | Результат |
| --- | --- |
| ESLint | пройден |
| TypeScript `tsc --noEmit` | пройден |
| Vitest | 45 файлов, 257 тестов |
| Frontend coverage | statements 95.26%, branches 90.02%, functions 93.01%, lines 98.35% |
| OpenAPI lint | оба контракта валидны |
| Next.js production build | пройден, 19 маршрутов сгенерированы |
| Playwright | 32/32 |
| Cypress | 52/52, 133.82 s, без повторных попыток |
| Go race | пройден на Go 1.26.9 |
| Go integration coverage | statements 90.0% на PostgreSQL 17 |
| `govulncheck` v1.7.0 | 0 достижимых уязвимостей |
| `npm audit --omit=dev --audit-level=high` | 0 уязвимостей production-зависимостей |
| Docker build/migrations | пройдены |
| Docker smoke | landing 200, account 200, health 200, readiness 200 |

Подробный Cypress-отчёт: [summary.md](../../reports/cypress/summary.md).

## Исправления, сделанные в рамках gate

- Устранены ошибки UUID-cast и LIKE-экранирования в PostgreSQL-запросах.
- Добавлены интеграционные и негативные тесты репозиториев, обработчиков,
  мониторинга, SMTP и проектных уведомлений.
- Стабилизирована синхронизация Cypress со вторичной загрузкой страницы проекта
  в development-режиме React.
- Backend переведён с уязвимого Go 1.25.13 на Go 1.26.9.
- `golang.org/x/text` обновлён с 0.40.0 до 0.41.0.

## Воспроизводимые команды

```bash
npm run lint
npm run typecheck
npm run test:coverage
npm run lint:openapi
npm run build
npm run test:e2e
npm run test:ui
npm audit --omit=dev --audit-level=high

cd apps/api
GOTOOLCHAIN=go1.26.9 go test -race -count=1 ./...
GOTOOLCHAIN=go1.26.9 go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
```

Backend integration coverage выполнялся в `golang:1.26.9-alpine` в сети
локального Compose с изолированной БД `arhdesign_test`; отчёт формируется в
`apps/api/coverage.final.txt`, а сгенерированный файл не входит в поставку.

## Локальный стенд

Состав: Caddy, web, API, PostgreSQL 17, Mailpit и контейнер резервного
копирования. Все длительно работающие сервисы после пересборки находятся в
состоянии healthy; одноразовый `migrate` завершился успешно.

- Сайт: `http://localhost/`
- Кабинет: `http://localhost/account`
- Mailpit: `http://localhost:8026/`
- Health: `http://localhost/healthz`
- Readiness: `http://localhost/readyz`

Следующий отдельный этап — `R1-REL-001`: PR рабочей ветки в `develop`, затем PR
`develop` в `main`, тег `0.2.0` и production smoke/rollback verification.
