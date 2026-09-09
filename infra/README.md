# Docker deployment

## Local verification

1. Create a local runtime file with non-production values: `cp .env.example .env`.
2. Start the complete stack: `docker compose -f infra/docker-compose.yml up -d --build`.
3. Wait until `docker compose -f infra/docker-compose.yml ps` reports `web`, `api` and `postgres` as healthy and `migrate` as exited with code `0`.
4. Open `http://localhost/`, then verify `http://localhost/healthz` and `http://localhost/readyz`. The local `.env` uses HTTP to avoid a browser certificate warning; a VDS domain without the `http://` prefix enables Caddy TLS.
5. Stop the stack without deleting data: `docker compose -f infra/docker-compose.yml down`. Add `--volumes` only when intentionally discarding the local database.

The local `.env` is ignored by Git. It must contain only disposable local credentials and must never be copied to a server.

## VDS deployment

## Required server setup

- Install Docker Engine and Docker Compose plugin.
- Point the DNS A/AAAA records for the production domain to the VDS.
- Allow inbound TCP ports 80 and 443 only. PostgreSQL, Next.js and Go API are private Compose services.

## First deployment

1. Copy the repository to the VDS and create the runtime environment file: `cp .env.example .env`.
2. Set `CADDY_SITE` to the public domain and replace both secret placeholders with unique values. Do not commit `.env`.
3. Build and start the production-like stack: `docker compose -f infra/docker-compose.yml up -d --build`.
4. Check `https://<domain>/healthz` and `https://<domain>/readyz`. Caddy obtains and renews TLS certificates after DNS is active.

## Database migrations

The `migrate` service runs the idempotent SQL files from `apps/api/migrations/` before API startup. Check its result with `docker compose -f infra/docker-compose.yml logs migrate`.

## Backup and restore

Create a compressed logical backup:

```sh
docker compose -f infra/docker-compose.yml exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' | gzip > arhdesign-$(date +%F).sql.gz
```

Restore only during a maintenance window, after taking a fresh backup:

```sh
gzip -dc arhdesign-YYYY-MM-DD.sql.gz | docker compose -f infra/docker-compose.yml exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

## Rollback

Keep the previous image digest or Git revision. To roll back application code, deploy that revision and run `docker compose -f infra/docker-compose.yml up -d --build`. Database migrations must be backward compatible; destructive schema changes require a separately approved migration and backup/restore rehearsal.
