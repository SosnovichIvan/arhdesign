# Инфраструктура arhDesign

Полная актуальная инструкция по архитектуре, домену, Docker, VPS, GitHub Actions, Telegram и резервным копиям находится в корневом [README](../README.md).

Эта папка — источник инфраструктурной конфигурации:

- `docker-compose.yml` — сервисы production-контура;
- `Caddyfile` — HTTPS и маршрутизация;
- `cloudflare/telegram-relay.js` — Cloudflare Worker для доставки и удаления Telegram-уведомлений;
- `scripts/backup-postgres.sh` — ежедневная резервная копия PostgreSQL и автоматическая очистка старых архивов.

Сроки хранения задаются через `.env`: `CONTACT_RETENTION_DAYS` (заявки, по умолчанию 365 дней), `TELEGRAM_NOTIFICATION_RETENTION_HOURS` (копии заявок в Telegram, не более 24 часов) и `BACKUP_RETENTION_DAYS` (резервные копии, 30 дней). Access-логи Caddy ротируются и хранятся не более 30 дней.

Для запуска используйте `.env` из корня репозитория:

```sh
docker compose --env-file .env -f infra/docker-compose.yml up -d --build
docker compose --env-file .env -f infra/docker-compose.yml ps
```
