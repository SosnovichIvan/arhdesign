## Why

Согласованный дизайн Concept A нужно превратить в production-сайт на VDS с собственной контактной формой и управляемым backend-контуром.

## From → To

- **From:** согласованный Figma Concept A, контекст и правила; production-код отсутствует.
- **To:** Next.js portfolio, Go REST API для заявок, PostgreSQL и Docker Compose deployment на VDS.

## Scope

- Frontend всех утверждённых маршрутов и состояний Concept A.
- Собственная contact form → Go API → PostgreSQL → уведомления email/Telegram.
- Внешние ссылки: VK `https://vk.ru/studio_architecture_design`, Telegram `https://t.me/studio_architecture_design`.
- VDS-ready Docker Compose, reverse proxy, TLS-ready configuration, health checks, backup/rollback documentation.

## Non-goals

- CMS, личный кабинет, платежи, авторизация посетителей и редактирование контента через UI. Модель будущей операторской авторизации документируется в этом change, но не реализуется без отдельного approved change.
- Kubernetes, MFE, gRPC, NATS и production deploy без отдельного разрешения.

## Version

- Previous: отсутствует.
- Target: v1.0.0.
- Level: MAJOR — первый production пользовательский продукт и публичный API.
