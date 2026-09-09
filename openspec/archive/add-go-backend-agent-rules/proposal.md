## Why

arhDesign переходит к собственной форме и будущему API на VDS. Агентам нужны единые, не избыточные правила frontend, Go backend, API-контрактов, безопасности, тестирования и деплоя.

## What Changes

- Добавить Go backend skill и адаптированные проектные правила в `.ai/rules/`.
- Дополнить frontend и review skills правилами API и проверки backend-изменений.
- Зафиксировать VDS-ориентированную инфраструктуру: Docker Compose, reverse proxy, TLS и PostgreSQL.

## Non-goals

- Не создавать production API, инфраструктуру или внешний деплой.
- Не переносить MFE, gRPC, NATS, Kubernetes, Envoy и продуктовые правила my-sport-life.
