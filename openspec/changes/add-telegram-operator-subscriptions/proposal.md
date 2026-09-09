## Why

Контактные заявки сохраняются в PostgreSQL, но оператор пока не может самостоятельно и безопасно подписать свой Telegram-чат на уведомления. Настройки runtime не должны попадать в Git: они должны передаваться из GitHub Actions secrets при deploy проверенного тега из `main`.

## From → To

- **From:** Telegram adapter может отправлять сообщения только в один вручную заданный `TELEGRAM_CHAT_ID`; корневой `.env` не содержит notification settings, а deploy workflow отсутствует.
- **To:** Telegram bot принимает подписку доверенного оператора по заданным login/password, хранит только `chat_id` и статус подписки в PostgreSQL, и рассылает новые contact submissions всем активным подписчикам. Runtime configuration формируется из GitHub secrets при tag-based deployment.

## Scope

- Telegram webhook endpoint, команда подписки и PostgreSQL-модель подписчиков.
- Runtime credentials: `TELEGRAM_BOT_TOKEN`, `TELEGRAM_ADMIN_USERNAME`, `TELEGRAM_ADMIN_PASSWORD`; это единственная системная пара главного администратора, password никогда не хранится в БД или логах.
- Notification fan-out в активные Telegram chat IDs после сохранения contact submission.
- Базовая Docker Compose env mapping для локального запуска бота.

## Deferred TODO

- GitHub Actions deployment on tag push only when tagged commit is reachable from `main`; remote deployment must obtain all runtime values from GitHub secrets.
- Remote `.env` materialization, VPS webhook configuration and production health check.
- README release instructions for GitHub Secrets and a specific production domain. These need actual VPS/domain inputs and will be implemented in a separate approved delivery stage.

## Non-goals

- Посетительская регистрация, UI login, JWT/cookies, Telegram OAuth, operator dashboard и изменение контента.
- Хранение Telegram credentials в репозитории, запуск deploy при каждом push в `main`, либо автоматический production deploy без tag.
- Реальный deploy на VPS без отдельно предоставленных domain, server access и GitHub secrets.

## Context impact

После реализации обновятся `.ai/context/project-state.md`, `decision-log.md`, `feedback-log.md`, `README.md`, `infra/README.md`, OpenAPI contract и deployment documentation.
