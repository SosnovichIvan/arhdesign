# arhDesign — сайт-портфолио Светланы Полисмаковой

Production-сайт архитектора и дизайнера интерьеров. Репозиторий содержит лендинг и страницы проектов, API контактной формы, Telegram-бот для подписки на заявки, PostgreSQL и Docker-инфраструктуру для VPS.

Публичный адрес: [designer-svetlana.ru](https://designer-svetlana.ru/).

> Секреты, пароли, токены, содержимое `.env` и резервные копии базы не хранятся в Git и не должны попадать в чат, задачи или скриншоты.

## Как устроен сайт

```text
Посетитель
  │ HTTPS
  ▼
Caddy на VPS ───────────────► Next.js 16 (сайт)
  │                                  │
  └──── /api/* ─────────────► Go API │
                                      │
                               PostgreSQL
                                      │
                 уведомление о заявке │
                                      ▼
        Cloudflare Worker relay ───► Telegram Bot API ───► подписанные администраторы

Telegram Bot API ── webhook ──► bot.designer-svetlana.ru ──► Caddy ──► Go API
```

| Часть | Технология | Назначение |
| --- | --- | --- |
| Интерфейс | Next.js 16, TypeScript, Tailwind | Лендинг, проекты, темы, адаптивное меню, SEO и форма заявки |
| API | Go | Валидация и сохранение формы, антиспам, Telegram webhook |
| База | PostgreSQL 17 | Заявки, доказательства согласия, cooldown формы, подписки и реестр удаления Telegram-уведомлений |
| Reverse proxy | Caddy 2 | HTTPS, выпуск и продление сертификатов, маршрутизация на сайт/API |
| Контейнеры | Docker Compose | Изолированный запуск всех сервисов на VPS |
| CI/CD | GitHub Actions | Сборка и публикация релиза по тегу из `main` |
| DNS/edge | Cloudflare | DNS домена, защищённый relay и проксирование адреса бота |

## Функциональность

### Публичная часть

- Главная страница: hero, услуги, процесс работы, проекты, автор, контакты и единая модальная форма.
- Страницы `/projects` и `/projects/[slug]`, галерея изображений, полноэкранный просмотр и управление свайпом/мышью.
- Светлая и тёмная темы; значение темы сохраняется в браузере.
- Закреплённый header, адаптивное меню и плавная навигация к разделам главной страницы.
- SEO: `robots.txt`, `sitemap.xml`, метаданные и favicon.

### Контактная форма

Форма отправляет `POST /api/v1/contact-submissions`.

- Обязательные поля: имя, телефон или почта, тип проекта и согласие на обработку данных.
- «О проекте» — необязательное поле.
- Сервер повторно валидирует все данные; браузерной проверки недостаточно.
- Антиспам: скрытое поле-ловушка, лимит запросов с одного IP и часовой cooldown для отправителя.
- Заявка сохраняется в PostgreSQL **до** попытки отправки уведомления. Поэтому кратковременная ошибка Telegram не должна уничтожить заявку.
- Для каждой заявки в `contact_consents` сохраняются факт и способ согласия, точный текст, версия, путь и SHA-256 опубликованного PDF, URL страницы и серверное время; запись связана с заявкой внешним ключом.
- После сохранения Go API отправляет уведомление каждому активному Telegram-подписчику.
- Очистка устаревших заявок запускается при старте API и затем раз в сутки; текущий срок хранения по умолчанию — 365 дней.
- Отправленные в Telegram копии заявок регистрируются в базе и автоматически удаляются ботом не позднее чем через 24 часа.

### Telegram-бот

Бот нужен только администраторам сайта.

1. Администратор отправляет `/start` или `/menu`.
2. Бот показывает ровно одну доступную кнопку:
   - нет активной подписки — «Подписаться»;
   - есть активная подписка — «Отписаться».
3. При подписке бот последовательно запрашивает логин и пароль администратора.
4. При корректных данных в PostgreSQL сохраняется `chat_id`; пароль в базу не записывается.
5. При «Отписаться» чат перестаёт получать заявки, но запись сохраняется как неактивная.

Состояние ввода логина/пароля живёт в памяти API не более 5 минут. Если в этот момент выполнить рестарт API, достаточно снова начать с `/start`.

## Репозиторий

| Путь | Содержимое |
| --- | --- |
| `src/` | Next.js-маршруты, компоненты и стили сайта |
| `src/features/contactForm/` | Единая модальная форма заявки |
| `src/shared/config/` | Контент проектов, SEO и контакты |
| `apps/api/` | Go API, миграции PostgreSQL, Telegram и notification adapters |
| `openapi/contact-api.yaml` | Публичный контракт contact API |
| `infra/docker-compose.yml` | Production-стек Docker Compose |
| `infra/Caddyfile` | HTTPS и reverse proxy |
| `infra/cloudflare/telegram-relay.js` | Исходный код Cloudflare Worker для доставки уведомлений |
| `.github/workflows/deploy.yml` | Деплой release-тегов на VPS |
| `.github/workflows/quality.yml` | Проверки качества в GitHub Actions |

## Локальный запуск

### Требования

- Docker Engine и Docker Compose plugin;
- Node.js/npm — только для запуска фронтенд-проверок вне Docker;
- Go — только для запуска API-проверок вне Docker.

### Запуск полного контура

```sh
git clone git@github.com:SosnovichIvan/arhdesign.git
cd arhdesign
cp .env.example .env
docker compose --env-file .env -f infra/docker-compose.yml up -d --build
docker compose --env-file .env -f infra/docker-compose.yml ps
```

Откройте [http://localhost/](http://localhost/). Локальная конфигурация намеренно использует HTTP: доверенный development TLS-сертификат не требуется.

Полезные проверки:

```sh
curl --fail http://localhost/healthz
curl --fail http://localhost/readyz
docker compose --env-file .env -f infra/docker-compose.yml logs -f api
```

Остановка без удаления данных:

```sh
docker compose --env-file .env -f infra/docker-compose.yml down
```

Не используйте `down --volumes`, если нужно сохранить локальную базу.

## Production: что уже настроено

### VPS

Production работает на отдельном VPS. На хосте публичны только `80/tcp` и `443/tcp`; контейнеры Next.js, Go API и PostgreSQL не публикуют собственные порты наружу.

| Сервис Compose | Назначение | Доступ |
| --- | --- | --- |
| `caddy` | HTTPS и маршрутизация | публичный, 80/443 |
| `web` | Next.js-сайт | только внутренняя Docker-сеть |
| `api` | Go API и Telegram webhook | только через Caddy |
| `postgres` | данные заявок и подписок | только внутренняя Docker-сеть |
| `migrate` | выполняет SQL-миграции перед API | одноразовый контейнер |
| `postgres-backup` | ежедневно создаёт сжатую резервную копию PostgreSQL | только внутренняя Docker-сеть и отдельный volume |

Данные PostgreSQL находятся в именованном Docker volume `postgres_data`. Удалять этот volume нельзя без резервной копии.

### Подготовка нового VPS

Для переноса проекта на новый сервер потребуются Ubuntu/Debian-подобный VPS с публичным IPv4, доступом по SSH и правами `sudo`.

1. Установите Docker Engine и Docker Compose plugin по [официальной инструкции Docker](https://docs.docker.com/engine/install/).
2. Откройте только нужные входящие порты:

   ```sh
   sudo ufw allow OpenSSH
   sudo ufw allow 80/tcp
   sudo ufw allow 443/tcp
   sudo ufw enable
   ```

3. Создайте каталог, в который GitHub Actions будет выкладывать release:

   ```sh
   sudo mkdir -p /opt/arhdesign
   sudo chown "$USER":"$USER" /opt/arhdesign
   git clone https://github.com/SosnovichIvan/arhdesign.git /opt/arhdesign
   cd /opt/arhdesign
   cp .env.example .env
   chmod 600 .env
   ```

4. Создайте DNS-записи домена до первого запуска и добавьте значения конфигурации в GitHub repository secrets. Дальше production запускается release-тегом; вручную заполнять production `.env` не нужно.

### Домен, DNS и HTTPS

Текущая production-схема для `designer-svetlana.ru`:

| Имя | Тип | Значение | Режим | Зачем |
| --- | --- | --- | --- | --- |
| `designer-svetlana.ru` | `A` | публичный IPv4 VPS | DNS only | основной сайт доступен без Cloudflare proxy |
| `www.designer-svetlana.ru` | `A` | публичный IPv4 VPS | DNS only | альтернативный адрес сайта |
| `bot.designer-svetlana.ru` | `A` | публичный IPv4 VPS | Proxied | стабильный HTTPS webhook для Telegram |
| NS домена | Cloudflare nameservers | назначенные Cloudflare NS | — | Cloudflare управляет DNS-зоной |

Основной домен и `www` специально оставлены в режиме **DNS only**, чтобы не завязывать доступность сайта из России на CDN-proxy Cloudflare. Caddy на VPS получает и обновляет TLS-сертификаты автоматически через Let's Encrypt.

Для Telegram webhook в Cloudflare следует оставить режим SSL/TLS **Full (strict)**. При изменении DNS дождитесь распространения записей, прежде чем проверять сертификат.

### Caddy

В runtime `.env`:

```dotenv
CADDY_SITE=www.designer-svetlana.ru
NEXT_PUBLIC_SITE_URL=https://designer-svetlana.ru
TELEGRAM_WEBHOOK_HOST=bot.designer-svetlana.ru
```

Workflow добавляет `designer-svetlana.ru` как алиас к `CADDY_SITE`, поэтому Caddy обслуживает основной домен, `www` и hostname бота. `CADDY_SITE` содержит доменные имена **без** `https://`; `NEXT_PUBLIC_SITE_URL` — полный canonical URL с `https://`.

## Конфигурация и секреты

### Runtime `.env` на VPS

Файл находится в каталоге деплоя (сейчас `/opt/arhdesign/.env`), имеет права `600`, создаётся GitHub Actions и не коммитится.

| Переменная | Где используется | Для чего |
| --- | --- | --- |
| `CADDY_SITE` | Caddy | домены, для которых Caddy выпускает TLS |
| `NEXT_PUBLIC_SITE_URL` | сборка Next.js | canonical URL сайта и SEO |
| `POSTGRES_DB` | PostgreSQL/API | имя базы |
| `POSTGRES_USER` | PostgreSQL/API | пользователь базы |
| `POSTGRES_PASSWORD` | PostgreSQL/API | пароль базы |
| `COOLDOWN_HMAC_SECRET` | Go API | хеширование маркера cooldown формы |
| `CONTACT_RETENTION_DAYS` | Go API | срок хранения заявок и связанных согласий; по умолчанию 365 дней |
| `TELEGRAM_BOT_TOKEN` | Go API, Cloudflare Worker, deploy | доступ к Telegram Bot API |
| `TELEGRAM_WEBHOOK_HOST` | Caddy, deploy | поддомен endpoint’а `/api/telegram/webhook` |
| `TELEGRAM_WEBHOOK_SECRET` | Go API, Telegram | проверка, что webhook пришёл от Telegram |
| `TELEGRAM_NOTIFICATION_RETENTION_HOURS` | Go API | срок хранения копии заявки в Telegram; от 1 до 24 часов |
| `ADMIN_USERNAME` | Go API | логин администратора бота |
| `ADMIN_PASSWORD` | Go API | пароль администратора бота |
| `TELEGRAM_RELAY_URL` | Go API | URL Worker `/sendMessage` |
| `TELEGRAM_RELAY_SECRET` | Go API, Worker | авторизация API перед relay |
| `RELEASE_IMAGE_TAG` | Compose | точная версия Docker images для релиза |
| `BACKUP_RETENTION_DAYS` | backup-контейнер | срок хранения ежедневных архивов PostgreSQL; по умолчанию 30 дней |

Пример без реальных значений — [`.env.example`](.env.example). Для генерации нового криптографического секрета:

```sh
openssl rand -hex 32
```

### GitHub repository secrets

GitHub Actions использует следующие Repository secrets; их значения не выводятся в логи:

```text
ADMIN_PASSWORD
ADMIN_USERNAME
CADDY_SITE
COOLDOWN_HMAC_SECRET
NEXT_PUBLIC_SITE_URL
POSTGRES_DB
POSTGRES_PASSWORD
POSTGRES_USER
TELEGRAM_BOT_TOKEN
TELEGRAM_RELAY_SECRET
TELEGRAM_RELAY_URL
TELEGRAM_WEBHOOK_HOST
TELEGRAM_WEBHOOK_SECRET
VPS_DEPLOY_PATH
VPS_HOST
VPS_PASSWORD
VPS_PORT
VPS_SSH_AUTH_METHOD
VPS_USER
```

Текущий workflow ожидает `VPS_SSH_AUTH_METHOD=password`. Это рабочая схема, но для следующего этапа безопасности рекомендуется перейти на отдельного deploy-пользователя и SSH private key в GitHub Secret.

### Cloudflare Worker relay

Worker нужен только для исходящей доставки и последующего удаления уведомления о заявке. Он принимает JSON от Go API, проверяет заголовок `Authorization: Bearer <RELAY_SECRET>` и вызывает разрешённые методы Telegram Bot API: `sendMessage` и `deleteMessage`.

- Исходник: [`infra/cloudflare/telegram-relay.js`](infra/cloudflare/telegram-relay.js).
- URL API должен оканчиваться на `/sendMessage`.
- В Cloudflare Worker secrets хранятся `TELEGRAM_BOT_TOKEN` и `RELAY_SECRET`.
- В настройках Worker отключено сохранение invocation logs: текст заявок не должен оставаться в логах edge-платформы.
- В `.env`/GitHub `TELEGRAM_RELAY_SECRET` должен совпадать с Worker `RELAY_SECRET`.

Relay не хранит заявку: он передаёт только `chat_id`, текст уведомления и, при удалении, `message_id`. Сама заявка прежде сохраняется на VPS в PostgreSQL. Worker invocation logs должны оставаться отключёнными.

### Неактивные каналы

В API предусмотрен SMTP adapter для email-уведомлений, но в текущем production-контуре он не настроен и не используется: единственный активный канал уведомлений — Telegram через relay. Не добавляйте SMTP-переменные в GitHub secrets, пока не появится согласованная задача на email-канал.

## Как выполняется релиз

Production-деплой запускается только после push тега формата `v*`, который указывает на commit в ветке `main`.

```sh
git checkout main
git pull --ff-only
git tag v1.0.14
git push origin v1.0.14
```

Workflow [`deploy.yml`](.github/workflows/deploy.yml) выполняет следующее:

1. Проверяет, что tag-коммит входит в `main`.
2. Собирает неизменяемые Docker images фронтенда и API в GitHub Actions.
3. Передаёт images и runtime-конфигурацию на VPS по SSH.
4. На VPS получает ровно tag-коммит, записывает `.env` с правами `600`, загружает images и запускает `docker compose ... up -d --no-build --wait`.
5. Настраивает Telegram webhook и команды `/start`, `/menu` через Bot API.
6. Проверяет, что Telegram подтверждает ожидаемый URL webhook.

Изменение кода без нового tag не изменяет production. Тег можно создавать только после прохождения локальных и GitHub-проверок.

## Проверки качества

```sh
npm ci
npm run lint
npm run typecheck
npm run test:coverage
npm run test:e2e

(cd apps/api && go vet ./... && go test -race -count=1 ./...)
```

Go coverage с PostgreSQL требует отдельной базы, имя которой содержит `test`:

```sh
cd apps/api
TEST_DATABASE_URL=postgres://.../arhdesign_test?sslmode=disable bash scripts/check-coverage.sh
```

## Эксплуатация VPS

### Проверка состояния

```sh
cd /opt/arhdesign
docker compose --env-file .env -f infra/docker-compose.yml ps
docker compose --env-file .env -f infra/docker-compose.yml logs --since 30m api
docker compose --env-file .env -f infra/docker-compose.yml logs --since 30m caddy
docker compose --env-file .env -f infra/docker-compose.yml exec -T postgres sh -c 'pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
df -h /
docker system df
```

HTTP health checks:

```sh
curl --fail https://designer-svetlana.ru/healthz
curl --fail https://designer-svetlana.ru/readyz
```

### Диск и Docker cache

VPS имеет ограниченный диск. После нескольких релизов старый build cache может заполнить filesystem и остановить PostgreSQL. Проверяйте `df -h /` и `docker system df` после деплоя.

Безопасная очистка только неиспользуемого cache:

```sh
docker builder prune --all --force
```

Команда не удаляет работающие контейнеры и volume с базой, но кэш следующей сборки будет создан заново. Не запускайте массовое удаление Docker volumes без свежей резервной копии.

### Backup и восстановление PostgreSQL

Контейнер `postgres-backup` автоматически создаёт один сжатый логический backup в сутки в volume `postgres_backups` и удаляет архивы старше `BACKUP_RETENTION_DAYS` (по умолчанию 30 дней). Проверить архивы:

```sh
docker compose --env-file .env -f infra/docker-compose.yml exec -T postgres-backup ls -lh /backups
```

Создать дополнительную ручную копию:

```sh
cd /opt/arhdesign
docker compose --env-file .env -f infra/docker-compose.yml exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' | gzip > arhdesign-$(date +%F).sql.gz
```

Восстановление выполняйте только в окно обслуживания и после нового backup:

```sh
gzip -dc arhdesign-YYYY-MM-DD.sql.gz | docker compose --env-file .env -f infra/docker-compose.yml exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

### Rollback

Для кода вернитесь к предыдущему проверенному tag/commit и повторите запуск Compose с образами этого релиза. Схема миграций должна оставаться обратно совместимой; перед откатом с изменениями базы всегда создавайте backup.

## Персональные данные

Форма содержит персональные данные. Текущая техническая обработка:

1. Данные формы сохраняются в PostgreSQL на VPS в России. В `contact_submissions` находятся имя, телефон или email, тип и необязательное описание проекта. В `contact_consents` сохраняется проверяемое доказательство согласия: факт, способ, точный текст, версия, путь и SHA-256 документа, URL страницы и серверное время.
2. До отправки форма требует отдельного, заранее не отмеченного checkbox. Рядом доступны [политика обработки персональных данных](/privacy) и полный [текст согласия](public/documents/personal-data-consent-2026-09-19-v4.pdf), открываемый в новой вкладке.
3. Уведомление передаётся администратору через Cloudflare Worker в Telegram. Relay не хранит заявку, а сообщение в Telegram автоматически удаляется ботом не позднее чем через 24 часа.
4. Заявки и связанные согласия хранятся до 365 дней; резервные копии PostgreSQL и access-логи Caddy — до 30 дней; cooldown формы — один час. Сроки настраиваются через `.env` в пределах, разрешённых API.
5. При удалении заявки связанная запись согласия удаляется каскадно. Просроченные cooldown и Telegram receipts также очищаются автоматически.

Опубликованы:

- страница политики: [`/privacy`](src/app/privacy/page.tsx);
- PDF политики: [`personal-data-policy-2026-09-19-v1.pdf`](public/documents/personal-data-policy-2026-09-19-v1.pdf);
- PDF согласия: [`personal-data-consent-2026-09-19-v4.pdf`](public/documents/personal-data-consent-2026-09-19-v4.pdf);
- эксплуатационная инструкция: [`docs/operations/pii-and-retention.md`](docs/operations/pii-and-retention.md).

Остаются организационные действия вне автоматизации репозитория: проверить наличие оператора в реестре Роскомнадзора и отдельно подать уведомление о трансграничной передаче персональных данных.

## Быстрая диагностика проблем

| Симптом | Что проверить |
| --- | --- |
| Сайт не открывается | DNS `A`, порты 80/443, `docker compose ps`, логи Caddy |
| Не выпускается TLS | DNS уже указывает на VPS, домен не закрыт другим proxy, логи `caddy` |
| Форма отвечает ошибкой | `api` logs, доступность PostgreSQL, cooldown/rate limit |
| Заявка сохранилась, но Telegram молчит | число активных подписчиков, `TELEGRAM_RELAY_*`, Worker secrets, API logs |
| `/start` не отвечает | DNS/proxy `bot` hostname, webhook status Bot API, `TELEGRAM_WEBHOOK_SECRET`, логи API |
| PostgreSQL не принимает подключения | свободное место `df -h /`, логи `postgres`, Docker build cache |

## Источники истины

- [`openapi/contact-api.yaml`](openapi/contact-api.yaml) — публичный контракт формы.
- [`infra/docker-compose.yml`](infra/docker-compose.yml) — фактический runtime-стек.
- [`.github/workflows/deploy.yml`](.github/workflows/deploy.yml) — механизм release-деплоя.
- [`infra/cloudflare/telegram-relay.js`](infra/cloudflare/telegram-relay.js) — логика relay.
- [`docs/architecture/overview.md`](docs/architecture/overview.md) — архитектурные ориентиры.
- [Figma: arhDesign — Website Concepts & SDD](https://www.figma.com/design/7CasuBb4xJjm0UYTOGZElV) — утверждённый дизайн.
