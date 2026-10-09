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
| База | PostgreSQL 17 | Заявки, кабинеты, проекты, чаты, технические обращения, подписки и аудит |
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

Бот используется для подтверждения профиля, восстановления доступа и персональных
уведомлений кабинета. После регистрации пользователь выбирает Telegram на сайте,
а браузер открывает `https://t.me/<bot>?start=register`.

1. На сайте нет полей Telegram-логина или Telegram-пароля.
2. В приватном чате бот последовательно запрашивает логин и пароль учётной записи
   сайта. Парольное сообщение удаляется сразу после проверки и не записывается в БД.
3. Для корректной пары данных pending-профиль становится активным, а `chat_id`
   привязывается к пользователю. Старые email-ссылки подтверждения отзываются.
4. Привязанный пользователь может выбрать Telegram для восстановления доступа;
   одноразовая ссылка отправляется только в уже подтверждённый чат.
5. Для супер-администратора такая привязка также включает получение заявок
   публичной формы. Обычный пользователь получает только относящиеся к нему события.

Технический администратор — отдельная системная учётная запись с глобальной ролью
`technical_admin`. API создаёт её при старте из `TECH_ADMIN_USERNAME`,
`TECH_ADMIN_EMAIL` и `TECH_ADMIN_PASSWORD` и не меняет уже установленный пароль
при повторном старте. После обычной авторизации в приватном Telegram-боте эта
учётная запись подписывается на технические уведомления: необработанные ошибки
frontend, ответы API класса `5xx`, panic и обращения из виджета кабинета.
Ожидаемые ошибки валидации и доступа `4xx` учитываются в метриках, но не создают
Telegram-шторм. Содержимое запросов, cookie, query string, токены и тексты чатов
в техническое уведомление не попадают.

### Личный кабинет и чаты

- `/account/chats` содержит личные и именованные групповые чаты между активными
  пользователями; участник находится по логину или email.
- Глобальные чаты не предоставляют доступ к проекту. Общий и контекстные чаты
  проекта остаются отдельными областями данных и прав.
- Сообщение сначала фиксируется в PostgreSQL, после чего отдельная запись outbox
  ставит Telegram-доставку доступным подписанным участникам. Сбой Telegram не
  отменяет сообщение.
- Кнопка обратной связи в правом нижнем углу кабинета отправляет жалобу или
  пожелание техническому администратору. Пользователь предупреждён не включать
  пароли, платёжные реквизиты и другие секреты.

Диалог подтверждения действует не более 5 минут. После истечения времени или
перезапуска API пользователь заново открывает ссылку из сайта либо запускает
`/start register`. Групповые чаты для ввода учётных данных не поддерживаются.
Диагностика, secrets, fallback и политика ручного восстановления описаны в
[`docs/operations/telegram-account-auth-runbook.md`](docs/operations/telegram-account-auth-runbook.md).

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
docker compose --env-file .env -f infra/docker-compose.yml -f infra/docker-compose.local.yml up -d --build
docker compose --env-file .env -f infra/docker-compose.yml -f infra/docker-compose.local.yml ps
```

Откройте [http://localhost/](http://localhost/). Локальный overlay обязателен:
он включает account API, системный вход супер-администратора и Mailpit, не
используя production SMTP или Telegram-секреты. Без
`infra/docker-compose.local.yml` базовый production-compose при пустом локальном
`.env` запускает API с `ACCOUNT_AUTH_ENABLED=false`, поэтому маршруты регистрации
и входа возвращают `404`. Локальная конфигурация намеренно использует HTTP:
доверенный development TLS-сертификат не требуется.

Форма регистрации доступна на [http://localhost/register](http://localhost/register),
а перехваченные письма — в [http://localhost:8026](http://localhost:8026).
Telegram-ссылка в этом контуре использует тестовое имя бота и проверяет только
формирование перехода; для end-to-end проверки реального бота нужны его локальные
секреты. Значения ключей в overlay предназначены только для локальной разработки.

Полезные проверки:

```sh
curl --fail http://localhost/healthz
curl --fail http://localhost/readyz
docker compose --env-file .env -f infra/docker-compose.yml logs -f api
```

Остановка без удаления данных:

```sh
docker compose --env-file .env -f infra/docker-compose.yml -f infra/docker-compose.local.yml down
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
| `POSTGRES_MAX_CONNECTIONS`, `POSTGRES_SHARED_BUFFERS` | PostgreSQL | connection budget (80) и память shared buffers (1 ГБ) для стартовой VM |
| `COOLDOWN_HMAC_SECRET` | Go API | хеширование маркера cooldown формы |
| `CONTACT_RETENTION_DAYS` | Go API | срок хранения заявок и связанных согласий; по умолчанию 365 дней |
| `ACCOUNT_AUTH_ENABLED` | Go API | включает R1 account endpoints и email outbox; до релиза кабинета оставляется `false` |
| `PUBLIC_ORIGIN` | Go API | origin сайта для fragment-ссылок подтверждения почты |
| `ACCOUNT_TOKEN_HMAC_SECRET` | Go API | purpose-keyed HMAC одноразовых auth-токенов; минимум 32 случайных байта |
| `ACCOUNT_TOKEN_HMAC_KEY_VERSION` | Go API | версия HMAC-ключа для контролируемой ротации |
| `OUTBOX_ENCRYPTION_KEY` | Go API | 32-байтный AES-GCM ключ в unpadded base64 для шифрования email payload в БД |
| `OUTBOX_ENCRYPTION_KEY_VERSION` | Go API | версия ключа шифрования outbox |
| `MONITORING_TIMEZONE`, `MONITORING_REPORT_HOUR` | Go API | timezone и час суточной Telegram-сводки |
| `MONITORING_BACKUP_DIRECTORY` | Go API | read-only каталог marker последнего проверенного off-site backup |
| `SMTP_ADDRESS`, `SMTP_USERNAME`, `SMTP_PASSWORD` | Go API | SMTP transport писем регистрации и восстановления; порт `465` использует implicit TLS, остальные порты требуют STARTTLS перед авторизацией |
| `EMAIL_FROM` | Go API | отправитель писем кабинета |
| `EMAIL_TO` | Go API | получатель уведомлений публичной контактной формы |
| `TELEGRAM_BOT_TOKEN` | Go API, Cloudflare Worker, deploy | доступ к Telegram Bot API |
| `TELEGRAM_BOT_USERNAME` | Go API | публичное имя бота без `@`; для production — `polismakovaSvetlanaBot`, используется для deep link `https://t.me/polismakovaSvetlanaBot?start=register` |
| `TELEGRAM_WEBHOOK_HOST` | Caddy, deploy | поддомен endpoint’а `/api/telegram/webhook` |
| `TELEGRAM_WEBHOOK_SECRET` | Go API, Telegram | проверка, что webhook пришёл от Telegram |
| `TELEGRAM_NOTIFICATION_RETENTION_HOURS` | Go API | срок хранения копии заявки в Telegram; от 1 до 24 часов |
| `ADMIN_USERNAME` | Go API | логин администратора бота |
| `ADMIN_PASSWORD` | Go API | пароль администратора бота |
| `TECH_ADMIN_USERNAME` | Go API | логин единственной активной технической учётной записи; задаётся секретом |
| `TECH_ADMIN_EMAIL` | Go API | уникальная почта технической учётной записи |
| `TECH_ADMIN_PASSWORD` | Go API | отдельный пароль единственной системной учётной записи `technical_admin`; для неё действует исключение — минимум 14 символов, пароль не коммитится; обычная регистрация требует минимум 15 |
| `TELEGRAM_RELAY_URL` | Go API | URL Worker `/sendMessage` |
| `TELEGRAM_RELAY_SECRET` | Go API, Worker | авторизация API перед relay |
| `RELEASE_IMAGE_TAG` | Compose | точная версия Docker images для релиза |
| `BACKUP_LOCAL_RETENTION_DAYS`, `BACKUP_INTERVAL_SECONDS` | backup-контейнер | локальная ретенция (7 дней) и интервал (24 часа) |
| `OFFSITE_BACKUP_ENABLED` | backup-контейнер | обязательное включение Restic/S3 в production |
| `BACKUP_RESTIC_REPOSITORY`, `BACKUP_RESTIC_PASSWORD` | backup-контейнер | S3 repository и отдельный ключ клиентского шифрования Restic |
| `BACKUP_AWS_ACCESS_KEY_ID`, `BACKUP_AWS_SECRET_ACCESS_KEY` | backup-контейнер | ограниченные credentials выделенного backup bucket |
| `BACKUP_REMOTE_DAILY_RETENTION`, `BACKUP_REMOTE_MONTHLY_RETENTION` | backup-контейнер | удалённая ретенция: 30 ежедневных и 12 ежемесячных snapshots |

Пример без реальных значений — [`.env.example`](.env.example). Для генерации нового криптографического секрета:

```sh
openssl rand -hex 32
openssl rand -base64 32 | tr -d '=\n'
```

Вторая команда создаёт значение `OUTBOX_ENCRYPTION_KEY`. Account-модуль
включается только после одновременной настройки PostgreSQL, SMTP, `PUBLIC_ORIGIN`
и обоих ключей; частичная конфигурация при `ACCOUNT_AUTH_ENABLED=true` останавливает
API до начала обслуживания запросов.

### GitHub repository secrets

GitHub Actions использует следующие Repository secrets; их значения не выводятся в логи:

```text
ADMIN_PASSWORD
ADMIN_USERNAME
ACCOUNT_TOKEN_HMAC_KEY_VERSION
ACCOUNT_TOKEN_HMAC_SECRET
BACKUP_AWS_ACCESS_KEY_ID
BACKUP_AWS_SECRET_ACCESS_KEY
BACKUP_RESTIC_PASSWORD
BACKUP_RESTIC_REPOSITORY
CADDY_SITE
COOLDOWN_HMAC_SECRET
NEXT_PUBLIC_SITE_URL
OUTBOX_ENCRYPTION_KEY
OUTBOX_ENCRYPTION_KEY_VERSION
POSTGRES_DB
POSTGRES_PASSWORD
POSTGRES_USER
SMTP_ADDRESS
SMTP_USERNAME
SMTP_PASSWORD
EMAIL_FROM
EMAIL_TO
TELEGRAM_BOT_TOKEN
TELEGRAM_RELAY_SECRET
TELEGRAM_RELAY_URL
TELEGRAM_WEBHOOK_HOST
TELEGRAM_WEBHOOK_SECRET
TECH_ADMIN_EMAIL
TECH_ADMIN_PASSWORD
TECH_ADMIN_USERNAME
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

### SMTP REG.RU

Для доменной почты используется `mail.hosting.reg.ru:465` с обязательным
implicit TLS. `SMTP_USERNAME` и `EMAIL_FROM` содержат полный адрес ящика;
`SMTP_PASSWORD` хранится только в GitHub Repository secrets и runtime `.env`
с правами `600`. Значение `EMAIL_TO` определяет получателя заявок публичной
контактной формы и не влияет на письма регистрации и восстановления пароля.
Перед публикацией настройки проверяются тестовым письмом, но его успешный приём
SMTP-сервером не заменяет наблюдение за bounce/DMARC.

## Как выполняется релиз

Production-деплой запускается только после push неизменяемого тега формата
`MAJOR.MINOR.PATCH`, который указывает на commit в ветке `main`.

```sh
git checkout main
git pull --ff-only
git tag 0.2.0
git push origin 0.2.0
```

Workflow [`deploy.yml`](.github/workflows/deploy.yml) выполняет следующее:

1. Проверяет, что tag-коммит входит в `main`.
2. Собирает неизменяемые Docker images фронтенда, API и backup в GitHub Actions.
3. Передаёт images и runtime-конфигурацию на VPS по SSH.
4. На VPS получает ровно tag-коммит, загружает images и создаёт `.env.next` с правами `600`.
5. До миграций создаёт проверяемую локальную и зашифрованную S3-копию текущей БД; сбой backup прекращает релиз.
6. Атомарно активирует `.env`, применяет additive migrations и запускает `docker compose ... up -d --no-build --wait`.
7. Настраивает Telegram webhook и команды `/start`, `/menu` через Bot API.
8. Проверяет, что Telegram подтверждает ожидаемый URL webhook.

Полный preflight, migration, rollback и security runbook:
[`docs/operations/r1-deployment-runbook.md`](docs/operations/r1-deployment-runbook.md).

Изменение кода без нового tag не изменяет production. Тег можно создавать только после прохождения локальных и GitHub-проверок.

## Проверки качества

```sh
npm ci
npm run lint
npm run typecheck
npm run test:coverage
npm run test:ui
npm run test:qa
npm run test:e2e

(cd apps/api && go vet ./... && go test -race -count=1 ./...)
```

`test:ui` запускает Cypress и является обязательным gate для пользовательских
веток. Любая добавленная или изменённая интерактивная логика одновременно
добавляется в [`docs/qa/r1-ui-scenario-matrix.md`](docs/qa/r1-ui-scenario-matrix.md)
и покрывается Cypress: happy path, validation, server error, empty/fallback и
access-control состояния, если они достижимы. Component/unit-тесты дополняют,
но не заменяют браузерный сценарий.

После каждого headless-прогона Cypress создаются:

- `reports/cypress/results.json` — машиночитаемый результат;
- `reports/cypress/summary.md` — число обнаруженных и выполненных сценариев,
  доля успешных, общее время и длительность каждого сценария.

`npm run test:qa` последовательно формирует V8-coverage кода и Cypress-отчёт.
Показатель Cypress «покрытие исполнения UI-сценариев» не является покрытием
строк/ветвей: полнота набора сценариев контролируется матрицей ниже, а code
coverage — Vitest с порогом 90%. В GitHub Actions Cypress-отчёт и снимки ошибок
сохраняются артефактом `cypress-qa-report` на 14 дней даже при падении тестов.

Go coverage с PostgreSQL требует отдельной базы, имя которой содержит `test`:

```sh
cd apps/api
TEST_DATABASE_URL=postgres://.../arhdesign_test?sslmode=disable bash scripts/check-coverage.sh
```

## Навыки агентов, правила и детерминированная валидация

В проект установлены локальные навыки из Agent Skills Lab (исходный каталог на рабочей машине: `/Users/ivansosnovich/Documents/codex/skils`). Их собранные копии и manifest находятся в [`.agents/skills/`](.agents/skills/) и [`.agents/skills/.agent-skills-lab.json`](.agents/skills/.agent-skills-lab.json). Инструкции и одобренные снимки правил находятся в [`AGENTS.md`](AGENTS.md); при расхождении с общими рекомендациями приоритет имеют требования пользователя, права доступа и проверенная конфигурация репозитория.

### Обязательный порядок работы

Для **любой будущей задачи с кодом** сначала применяется `execution-state`. Он оценивает объём работы: короткая связная задача получает `passthrough` без служебных файлов, а многоэтапная работа ведётся через компактное состояние, semantic chunks, контрольные точки и именованные regression checks. Рабочие состояния в `.execution-state/` не коммитятся; в них нельзя записывать секреты, пароли или персональные данные.

После маршрутизации используется профильный skill. Файлы меняются только после чтения соответствующего `SKILL.md`, локальных инструкций, актуальной документации Next.js из `node_modules/next/dist/docs/` и существующих аналогов в коде.

### Установленные skills

| Skill | Профиль проекта | Когда применять |
| --- | --- | --- |
| `execution-state` 1.0.0 | Codex, policy `all_tasks`, enforcement `instructions` | Любая задача перед реализацией или иной мутацией. |
| `ui-ux-design` 0.2.0 | UI, accessibility, responsive, states | Проектирование и изменение интерфейсов. |
| `frontend-engineering` 0.1.0 | TypeScript, React, Next.js 16, flexible logic-based | Код лендинга и других Next.js-интерфейсов. |
| `frontend-review` 0.1.0 | Frontend review | Независимое ревью изменений интерфейса. |
| `backend-engineering` 0.1.0 | Go | Go API, Telegram, OpenAPI-интеграции и фоновые процессы. |
| `backend-review` 0.1.0 | Backend review | Ревью API-контрактов, авторизации, отказов и конкурентности. |
| `database-engineering` 0.1.0 | PostgreSQL | SQL, миграции, индексы и backfill. |
| `database-review` 0.1.0 | PostgreSQL review | Независимое ревью миграций, изоляции и rollout. |
| `project-architecture` 0.1.0 | Architecture | Границы модулей, ADR, контракты и план перехода. |

Активированы проектные правила для UI/UX, backend, PostgreSQL, архитектуры, backend/database review и delivery: контракты и права проверяются на сервере, миграции не переписываются после применения, релиз идёт через `develop` → `main` и тег. Правила `frontend-engineering` и `frontend-review` намеренно не активированы: исходный набор требует FSD, а текущая подтверждённая структура проекта — гибкая `src/app`, `src/features`, `src/shared`. Это не повод для скрытой миграции; FSD можно включить только отдельной согласованной задачей.

### Валидаторы

Локальный CLI `@sosnovich/agent-skills` из `/Users/ivansosnovich/Documents/codex/skils/packages/agent-skills-cli` установил конфигурацию в [`.validation/`](.validation/). Контекст был подтверждён для `src`, `apps/api`, [`openapi/contact-api.yaml`](openapi/contact-api.yaml) и PostgreSQL.

| Валидатор | Правило | Статус и ограничение |
| --- | --- | --- |
| `test` | Для изменённого исходного файла ищет colocated-тест с суффиксом `.test.ts`, `.test.tsx`, `.spec.ts`, `.spec.tsx` или `_test.go`; целевой порог coverage — 90% lines, branches и changed lines. | Проверка наличия теста работает. Сам CLI не читает coverage-отчёты, поэтому результат в нём остаётся `not_verified`; реальный V8 coverage подключён адаптером `frontend-vitest-v8`, а Go coverage — `go-api-quality`. |
| `frontend` | Проверяет kebab-case имён исходных frontend-файлов в `src` и запрещает `eval(` и `dangerouslySetInnerHTML`. | Это лексический прототип, не AST-анализатор; его нельзя использовать как единственный критерий безопасности. |
| `backend` | Фиксирует область `apps/api`, контракт OpenAPI и PostgreSQL как входы для backend-проверок. | Парсеры OpenAPI и PostgreSQL в версии CLI 0.1.0 ещё не подключены, поэтому его собственный вывод имеет статус `not_verified`; реальные адаптеры проекта перечислены ниже и обязательны в CI. |

### Подключённые адаптеры проекта

Конфигурация находится в [`.validation/adapters.json`](.validation/adapters.json), а единая точка запуска — [`scripts/validate-adapters.sh`](scripts/validate-adapters.sh). Эти адаптеры выполняют реальные команды, а не только записывают технологию в context-файл.

| Адаптер | Что подтверждает | Команда |
| --- | --- | --- |
| `frontend-vitest-v8` | ESLint, TypeScript и V8 coverage для `src`; Vitest сам отклоняет покрытие ниже 90%. | `npm run validate:frontend-adapter` |
| `openapi-go-codegen` | OpenAPI-контракт генерирует bindings через `oapi-codegen` 2.5.0 без незафиксированного drift. | `npm run validate:openapi-adapter` |
| `postgresql-migrations` | Все миграции применяются на изолированном PostgreSQL в интеграционном тесте. | `TEST_DATABASE_URL=postgres://.../arhdesign_test?sslmode=disable npm run validate:postgres-adapter` |
| `go-api-quality` | `go vet`, race-тесты, PostgreSQL-мigrations, Go coverage не ниже 90% и OpenAPI. | `TEST_DATABASE_URL=postgres://.../arhdesign_test?sslmode=disable npm run validate:api-adapter` |

`npm run validate:adapters` запускает оба контура и требует `TEST_DATABASE_URL`. GitHub Actions использует те же команды: frontend-job вызывает `validate:frontend-adapter`, API-job — `validate-adapters.sh api`. Поэтому pull request и локальная проверка используют одинаковые правила.

Перед проверкой изменений обновляйте контекст только при изменении структуры проекта или правил. Команда не угадывает базовую ветку:

```sh
node /Users/ivansosnovich/Documents/codex/skils/packages/agent-skills-cli/bin/agent-skills.js \
  check --project . --base origin/develop --head HEAD --format markdown
```

Конфигурационные файлы `.validation/config.json`, `.validation/project-context.json` и `.validation/rules.json` следует коммитить вместе с проектом. Служебные receipts и резервные копии установщика из `.agent-skills-lab/runtime/` и `.agent-skills-lab/backups/` намеренно игнорируются Git.

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

Текущий backup хранится на том же VPS и остаётся быстрым локальным уровнем.
Целевая схема с зашифрованной off-site копией, point-in-time recovery начиная
с финансового релиза и обязательными restore drill описана в
[`docs/operations/postgresql-backup-and-recovery.md`](docs/operations/postgresql-backup-and-recovery.md).

Контейнер `postgres-backup` автоматически создаёт `pg_dump -Fc`, SHA-256 и
зашифрованный Restic snapshot в независимом S3. Локально хранятся 7 суток,
удалённо — 30 ежедневных и 12 ежемесячных snapshots. Проверить архивы:

```sh
docker compose --env-file .env -f infra/docker-compose.yml exec -T postgres-backup ls -lh /backups
```

Запустить дополнительную копию штатным механизмом:

```sh
docker compose --env-file .env -f infra/docker-compose.yml run --rm \
  -e BACKUP_RUN_ONCE=true postgres-backup
```

Восстановление выполняется только в отдельную PostgreSQL по инструкции
[`docs/operations/postgresql-backup-and-recovery.md`](docs/operations/postgresql-backup-and-recovery.md).

### Rollback

Используйте раздел Rollback в
[`docs/operations/r1-deployment-runbook.md`](docs/operations/r1-deployment-runbook.md).
Откат приложения не запускает destructive down migration; восстановление данных
выполняется только в новый экземпляр PostgreSQL.

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
- [`docs/architecture/client-cabinet-release-plan.md`](docs/architecture/client-cabinet-release-plan.md) — семь версий личного кабинета, пользовательские пути, Cypress и quality gates.
- [`docs/releases/client-cabinet/README.md`](docs/releases/client-cabinet/README.md) — исполняемые списки задач `0.2.0`–`0.8.0` со стабильными ID, зависимостями и приёмкой.
- [`docs/architecture/client-cabinet-capacity-plan.md`](docs/architecture/client-cabinet-capacity-plan.md) — стартовая VM VDSina, расчётные допущения, пороги масштабирования и нагрузочная приёмка.
- [`docs/architecture/client-cabinet-r1-auth-security-adr.md`](docs/architecture/client-cabinet-r1-auth-security-adr.md) — обязательная R1-политика паролей, токенов, cookie-сессий, CSRF и матрица security-тестов.
- [`docs/architecture/client-cabinet-r1-auth-abuse-audit-contract.md`](docs/architecture/client-cabinet-r1-auth-abuse-audit-contract.md) — rate limits, allowlisted security audit, отзыв доступа и конкурентная защита последнего супер-администратора.
- [`docs/architecture/client-cabinet-r1-monitoring-telegram-contract.md`](docs/architecture/client-cabinet-r1-monitoring-telegram-contract.md) — 15-минутные метрики, точные суточные агрегаты, Telegram-получатели, alert state machine и PII exclusions.
- [`docs/operations/r1-additive-migration-plan.md`](docs/operations/r1-additive-migration-plan.md) — порядок R1 migration, совместимость old/new, блокировки, PostgreSQL 17 tests и восстановление.
- [`docs/operations/telegram-account-auth-runbook.md`](docs/operations/telegram-account-auth-runbook.md) — Telegram confirmation/reset, diagnostics и безопасный fallback.
- [`docs/operations/postgresql-backup-and-recovery.md`](docs/operations/postgresql-backup-and-recovery.md) — целевая backup-схема, RPO/RTO, PITR и восстановление PostgreSQL.
- [Figma: arhDesign — Website Concepts & SDD](https://www.figma.com/design/7CasuBb4xJjm0UYTOGZElV) — утверждённый дизайн.
- [Страницы и сценарии личного кабинета](docs/design/client-cabinet-pages.md) — карта экранов, навигация, состояния и связи разделов.
