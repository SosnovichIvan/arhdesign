# VDS and deployment

v1 разворачивается на VDS через Docker Compose: `web`, `api`, `postgres` и reverse proxy (Caddy или Nginx). TLS и public ingress завершаются в proxy.

- Образы минимальны и воспроизводимы; secrets не попадают в image, compose file или git.
- В production наружу публикуется только proxy; API и PostgreSQL находятся во внутренней сети Compose.
- Для API обязательны healthcheck, restart policy, миграции и документированная процедура backup/restore PostgreSQL.
- Деплой, DNS, TLS и доступ к VDS требуют отдельного явного разрешения пользователя.
