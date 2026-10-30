# Инфраструктура arhDesign

Полная актуальная инструкция по архитектуре, домену, Docker, VPS, GitHub Actions, Telegram и резервным копиям находится в корневом [README](../README.md).

Эта папка — источник инфраструктурной конфигурации:

- `docker-compose.yml` — сервисы production-контура;
- `Caddyfile` — HTTPS и маршрутизация;
- `cloudflare/telegram-relay.js` — Cloudflare Worker для доставки и удаления Telegram-уведомлений;
- `backup/Dockerfile` и `scripts/backup-postgres.sh` — custom-format backup PostgreSQL, checksum и клиентски зашифрованный Restic/S3 snapshot;
- `scripts/restore-drill.sh` и `docker-compose.backup-drill.yml` — изолированная тренировка восстановления с измерением RPO/RTO.

Сроки хранения задаются через `.env`: `CONTACT_RETENTION_DAYS` (заявки, по умолчанию 365 дней), `TELEGRAM_NOTIFICATION_RETENTION_HOURS` (копии заявок в Telegram, не более 24 часов), `BACKUP_LOCAL_RETENTION_DAYS` (7 дней), `BACKUP_REMOTE_DAILY_RETENTION` (30) и `BACKUP_REMOTE_MONTHLY_RETENTION` (12). Access-логи Caddy ротируются и хранятся не более 30 дней.

Для запуска используйте `.env` из корня репозитория:

```sh
docker compose --env-file .env -f infra/docker-compose.yml up -d --build
docker compose --env-file .env -f infra/docker-compose.yml ps
```
