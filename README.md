# arhDesign

Production-портфолио архитектора-дизайнера Полисмаковой Светланы. В репозитории находятся Next.js-сайт, Go API для контактных заявок, PostgreSQL и Docker Compose-контур с Caddy.

## Состав

- `src/` — Next.js 16 App Router, TypeScript, семантические Light/Dark-токены и интерфейс Concept A.
- `apps/api/` — Go REST API; OpenAPI-контракт, серверная валидация, cooldown, rate limit, PostgreSQL и изолированные email/Telegram adapters.
- `openapi/contact-api.yaml` — единственный источник публичного API-контракта.
- `infra/` — Caddy, Docker Compose и инструкция эксплуатации.
- `openspec/changes/launch-portfolio-with-go-api/` — принятая спецификация и task ledger.

## Локальный запуск в Docker

Требуются Docker Engine и Docker Compose plugin.

```sh
cp .env.example .env
docker compose -f infra/docker-compose.yml up -d --build
docker compose -f infra/docker-compose.yml ps
```

После успешных healthchecks откройте `http://localhost/`. Локальный контур использует HTTP, чтобы не требовать доверия к development TLS certificate. Для VDS задайте домен без `http://` — Caddy включит TLS автоматически. Проверка API: `http://localhost/healthz` и `http://localhost/readyz`.

Остановка без удаления базы: `docker compose -f infra/docker-compose.yml down`.

## Развёртывание на VPS

Нужны VPS с публичным IPv4, домен и доступ по SSH с правами `sudo`. На сервере будут публичны только порты `80` и `443`: PostgreSQL, Next.js и Go API остаются во внутренней Docker-сети.

1. В DNS создайте запись `A` для домена (например, `example.com`) на публичный IP VPS. Если используется `www`, добавьте для него отдельную запись. Дождитесь распространения DNS до запуска Caddy.
2. Установите Docker Engine и Docker Compose plugin по официальной инструкции Docker. Откройте firewall:

   ```sh
   sudo ufw allow OpenSSH
   sudo ufw allow 80/tcp
   sudo ufw allow 443/tcp
   sudo ufw enable
   ```

3. Склонируйте проект и создайте runtime-конфигурацию. Файл `.env` содержит секреты, не добавляйте его в Git и не пересылайте в чат:

   ```sh
   git clone git@github.com:SosnovichIvan/arhdesign.git /opt/arhdesign
   cd /opt/arhdesign
   cp .env.example .env
   chmod 600 .env
   openssl rand -hex 32
   ```

4. Откройте `.env` и укажите реальные значения:

   ```dotenv
   CADDY_SITE=example.com
   NEXT_PUBLIC_SITE_URL=https://example.com
   POSTGRES_DB=arhdesign
   POSTGRES_USER=arhdesign
   POSTGRES_PASSWORD=<длинный-уникальный-пароль>
   COOLDOWN_HMAC_SECRET=<результат-openssl-rand-hex-32>
   TELEGRAM_WEBHOOK_HOST=bot.example.com
   TELEGRAM_RELAY_URL=https://arhdesign-telegram-relay.example.workers.dev/sendMessage
   TELEGRAM_RELAY_SECRET=<длинный-уникальный-ключ>
   ```

   `CADDY_SITE` — домен сайта без `https://`; Caddy автоматически выпустит и будет обновлять TLS-сертификат. `NEXT_PUBLIC_SITE_URL` — полный публичный URL с `https://`. Для Telegram создайте отдельную proxied DNS-запись `bot` и укажите её в `TELEGRAM_WEBHOOK_HOST`; записи основного домена и `www` могут работать в режиме DNS only.

   Если VPS не может устанавливать исходящие соединения с Telegram, разверните Worker из [`infra/cloudflare/telegram-relay.js`](infra/cloudflare/telegram-relay.js). В настройках Worker создайте зашифрованные секреты `TELEGRAM_BOT_TOKEN` и `RELAY_SECRET`, отключите сохранение invocation logs, затем укажите URL `/sendMessage` и тот же `RELAY_SECRET` в переменных `TELEGRAM_RELAY_URL` и `TELEGRAM_RELAY_SECRET` API. Relay принимает только авторизованные JSON-запросы, не хранит тело заявки и передаёт в Telegram только идентификатор чата и текст уведомления.

5. Соберите и запустите сервисы:

   ```sh
   docker compose --env-file .env -f infra/docker-compose.yml up -d --build
   docker compose --env-file .env -f infra/docker-compose.yml ps
   curl --fail https://example.com/healthz
   curl --fail https://example.com/readyz
   ```

   Статус `web`, `api` и `postgres` должен быть `healthy`, а `migrate` — завершиться с кодом `0`. Если TLS не выпустился, сначала проверьте DNS и логи: `docker compose --env-file .env -f infra/docker-compose.yml logs caddy`.

### GitHub Actions secrets

Для автоматического релиза по тегу `v*` добавьте в Repository secrets значения из runtime-конфигурации, включая `CADDY_SITE`, `NEXT_PUBLIC_SITE_URL`, `TELEGRAM_WEBHOOK_HOST`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`, `TELEGRAM_RELAY_URL`, `TELEGRAM_RELAY_SECRET`, `ADMIN_USERNAME`, `ADMIN_PASSWORD`, параметры PostgreSQL и параметры подключения к VPS. Workflow передаёт эти значения на сервер через временный файл, устанавливает для `.env` права `600` и не добавляет его в Git.

### Персональные данные

Контактная форма сначала сохраняет заявку в PostgreSQL на VPS, а relay используется только для доставки уведомления администратору. В интерфейсе формы прямо указано, что уведомление технически проходит через Cloudflare в Telegram. Перед публичным сбором заявок оператору сайта следует опубликовать отдельную политику обработки персональных данных с реквизитами оператора, составом данных, целями, сроками хранения, порядком отзыва согласия и перечнем внешних обработчиков. Использование зарубежной инфраструктуры также требует отдельно проверить обязанности по трансграничной передаче; одного текста у чекбокса для этого недостаточно.

### Обновление, backup и rollback

Перед обновлением сделайте backup базы, затем обновите Git-ревизию и пересоберите контейнеры:

```sh
cd /opt/arhdesign
docker compose --env-file .env -f infra/docker-compose.yml exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' | gzip > arhdesign-$(date +%F).sql.gz
git fetch origin
git checkout <проверенный-тег-или-коммит>
docker compose --env-file .env -f infra/docker-compose.yml up -d --build
```

Для rollback вернитесь к предыдущему проверенному Git commit/tag и повторите `up -d --build`. Миграции должны быть backward-compatible; перед восстановлением базы остановите внешнюю запись и сохраните свежий backup. Полная операционная инструкция находится в [`infra/README.md`](infra/README.md).

## Проверки

```sh
npm ci
npm run lint
npm run typecheck
npm run test:coverage
npm run test:e2e

(cd apps/api && go vet ./... && go test -race -count=1 ./...)
```

Go coverage с PostgreSQL требует изолированную базу, имя которой содержит `test`:

```sh
cd apps/api
TEST_DATABASE_URL=postgres://.../arhdesign_test?sslmode=disable bash scripts/check-coverage.sh
```

## Источники истины

- [`openspec/changes/launch-portfolio-with-go-api/`](openspec/changes/launch-portfolio-with-go-api/) — scope, architecture и ход выполнения.
- [`docs/architecture/overview.md`](docs/architecture/overview.md) — архитектура.
- [Figma: arhDesign — Website Concepts & SDD](https://www.figma.com/design/7CasuBb4xJjm0UYTOGZElV) — утверждённый дизайн Concept A.
