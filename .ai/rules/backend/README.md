# Backend — Go REST API

Использовать Go для REST API arhDesign. v1 — модульный монолит, не микросервисная платформа.

- Слои: `handler → service → repository`; зависимости собираются вручную в `cmd/api`.
- OpenAPI является контрактом. DTO генерируются и не редактируются вручную.
- PostgreSQL доступен только через repository; SQL параметризован, миграции версионированы.
- Config и secrets — через env, fail-fast на обязательной конфигурации.
- Graceful shutdown, `/healthz` и `/readyz` обязательны.
- Сторонние отправки (email, Telegram) имеют timeout и не блокируют HTTP-ответ бесконечно.

Запрещены прямой доступ handler к БД, ручные контрактные DTO и бизнес-логика в HTTP handlers.
