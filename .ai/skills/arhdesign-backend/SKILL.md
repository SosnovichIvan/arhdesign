---
name: arhdesign-backend
description: Реализовывать Go REST API arhDesign для формы и будущих сервисов на VDS. Использовать для production backend, OpenAPI, PostgreSQL и Docker Compose.
---

# Backend arhDesign

## Перед реализацией

- Прочитать `projectrules.md`, активный OpenSpec change, `.ai/context/` и применимые `.ai/rules/`.
- Не писать production-код до валидации и явного approval OpenSpec.
- Для UI-потоков сверять утверждённый Figma; для API оформить контракт и сценарии ошибок.

## Архитектура v1

Backend — один Go REST API рядом с Next.js приложением. Деплой на VDS выполняется Docker Compose за reverse proxy с TLS. PostgreSQL используется для заявок и будущих данных.

```text
apps/api/
  cmd/api/             # composition root, config, graceful shutdown
  internal/handler/    # HTTP, parsing, validation, response mapping
  internal/service/    # бизнес-правила
  internal/repository/ # PostgreSQL и внешние адаптеры
  internal/model/      # доменные типы
  internal/platform/   # config, log, http, telemetry
openapi/               # source of truth REST contract
infra/                 # Docker Compose, proxy, deployment docs
```

Соблюдать зависимость `handler → service → repository`. Handler не работает с БД напрямую; secrets доступны только через окружение; внешние email/Telegram клиенты изолированы в repository/adapter.

## Обязательные правила

- Contract-first: сначала `openapi/`, затем генерация Go/TypeScript типов, затем код.
- Контекст и таймауты передавать через все слои. Ошибки оборачивать через `%w`; наружу отдавать только согласованный API error.
- Форма: серверная валидация, rate limit, honeypot/anti-bot, аудит без содержимого персональных данных, idempotent обработка.
- Логи — структурированные; health/readiness endpoints обязательны. Не логировать телефон, email, текст заявки или секреты целиком.
- Тесты — table-driven; перед сдачей `go test -race -count=1 ./...`, `go vet ./...`, atomic statement coverage >=90% и явное покрытие всех decision branches, а также релевантные integration tests.

## Подробнее

- [Контракты](../../rules/contracts/README.md)
- [Безопасность](../../rules/security/README.md)
- [Тестирование](../../rules/testing/README.md)
- [VDS и деплой](../../rules/devops/README.md)
