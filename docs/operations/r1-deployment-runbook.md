# R1 deployment, rollback and security runbook

Статус: действующий для релиза `0.2.0`.

## 1. Источники конфигурации

Production secrets хранятся только в GitHub Repository secrets. Workflow
создаёт `/etc/arhdesign/production.env` с владельцем `root:arhdesign` и правами
`0640`; доступ ограничен root и deploy-пользователем в системной группе
`arhdesign`. Ручного редактирования файла во время штатного релиза нет. Полный
шаблон без значений находится в
`.env.example`, а единственный runtime contract — в `infra/docker-compose.yml`.

Обязательные группы секретов:

- Yandex Cloud: `YANDEX_HOST`, `YANDEX_SSH_PORT`, `YANDEX_SSH_USER`,
  `YANDEX_DEPLOY_SSH_KEY`;
- домен и БД: `CADDY_SITE`, `NEXT_PUBLIC_SITE_URL`, `POSTGRES_DB`,
  `POSTGRES_USER`, `POSTGRES_PASSWORD`;
- auth crypto: `COOLDOWN_HMAC_SECRET`, `ACCOUNT_TOKEN_HMAC_SECRET`,
  `ACCOUNT_TOKEN_HMAC_KEY_VERSION`, `OUTBOX_ENCRYPTION_KEY`,
  `OUTBOX_ENCRYPTION_KEY_VERSION`;
- SMTP и Telegram: переменные из `.env.example`, включая bot/relay/webhook;
- системные учётные записи: `ADMIN_*` и `TECH_ADMIN_*`;
- backup: на аварийном single-node этапе — локальный PostgreSQL dump и Yandex
  Cloud Backup; offsite Restic возвращается отдельным изменением после выбора
  независимого хранилища.

`ACCOUNT_TOKEN_HMAC_SECRET`, `OUTBOX_ENCRYPTION_KEY` и обязательные runtime
secrets проверяются генератором env до SSH. Пустое значение останавливает релиз.

## 2. Preflight релиза

1. PR рабочей ветки в `develop` прошёл lint, typecheck, unit/integration,
   coverage, Cypress, Playwright и migration checks.
2. PR `develop → main` прошёл те же обязательные checks и review.
3. Release commit находится в актуальной `origin/main`.
4. Тег имеет неизменяемый формат `MAJOR.MINOR.PATCH`, для R1 — `0.2.0`.
5. Последний Cloud Backup доступен; для существующей БД проверен свежий
   логический dump и сценарий восстановления.
6. На ВМ не менее 30% свободного диска, PostgreSQL healthy, активных incidents
   нет, SMTP и Telegram diagnostics зелёные.

Workflow дополнительно проверяет принадлежность tag commit ветке `main`.

## 3. Порядок deployment

1. GitHub Actions собирает web, API, Telegram proxy и backup images с тегом
   commit SHA.
2. Images и будущий `production.env` передаются на production-ВМ Yandex Cloud.
3. При существующей БД до миграций запускается одноразовый локальный backup;
   ошибка dump/checksum останавливает deployment.
4. `production.env` устанавливается в `/etc/arhdesign` с владельцем
   `root:arhdesign` и mode `0640`.
5. Single-node Compose запускает PostgreSQL, применяет additive migrations,
   затем API, web и Caddy; `--wait` требует успешных health checks.
6. API запускает Telegram long polling через закрытый SSH proxy; внешний webhook
   не используется.
7. Оператор проверяет `/healthz`, `/readyz`, вход, список проектов, Mail/Telegram
   diagnostics и отсутствие нового critical incident.

## 4. Migration safety

Миграции R1 additive и повторяемые. Детальная матрица old/new application ×
schema, блокировки и recovery описаны в `r1-additive-migration-plan.md`.
Миграция выполняется до запуска новой версии API. Ошибка SQL откатывает её
транзакцию и не позволяет API стать ready. Применённую миграцию не редактируют.

Перед production запуском фиксируются версия PostgreSQL, число строк в
затрагиваемых таблицах, длительность migration и lock waits. При неожиданном
размере/ожидании блокировки deployment отменяется и изменение индекса выносится
в отдельно рассмотренный совместимый этап.

## 5. Rollback приложения

Additive R1 schema совместима с предыдущим приложением, поэтому штатный rollback
не удаляет таблицы и не запускает down migration.

1. Остановить дальнейшие deployment и зафиксировать incident time.
2. Выбрать предыдущий проверенный тег, входящий в `main`, и его commit SHA.
3. Убедиться, что соответствующие images присутствуют на VPS; если нет —
   повторно доставить их из доверенного release artifact.
4. Сгенерировать env с предыдущим `RELEASE_IMAGE_TAG`, сохранив текущие
   crypto/DB credentials.
5. Выполнить `docker compose ... up -d --no-build --wait`.
6. Проверить health, login, чтение существующих данных и Telegram/SMTP.

Если повреждены данные, rollback кода недостаточен: восстановить backup в новую
PostgreSQL, проверить целостность и только затем переключить приложение. Нельзя
восстанавливать поверх единственного production volume.

## 6. Security incidents и ротация

- При компрометации session/auth secret: отключить account traffic, отозвать
  все sessions, добавить новый key version, развернуть и только затем удалить
  старый ключ после истечения совместимого окна.
- При компрометации `OUTBOX_ENCRYPTION_KEY`: остановить worker, сохранить старый
  ключ для чтения уже созданных записей, выпустить новую версию ключа и выполнить
  отдельно проверенный re-encryption/backfill.
- При компрометации Telegram/SMTP: отозвать credential у провайдера, заменить
  GitHub Secret, повторно развернуть и проверить diagnostics. Секрет не
  пересылается в чат и не попадает в issue/log.
- При компрометации S3: отозвать access key, проверить object audit, создать
  новый restricted key. Потеря Restic password без защищённой аварийной копии
  делает backup невосстановимым.
- Пароли пользователей, токены, cookie, DSN, тексты сообщений и PII запрещены в
  application/deploy logs. Техническая диагностика содержит только allowlisted
  identifiers, категории результата и агрегаты.

После ротации старый secret удаляется из GitHub только после успешного smoke и
проверки, что ни один активный process его не использует.

## 7. Release evidence

К релизу прикладываются: ссылки на два PR, commit/tag, quality report, migration
output, backup snapshot/checksum и restore metrics, load/capacity report,
production health, Telegram webhook diagnostics и решение go/no-go. Любой
непройденный обязательный пункт означает no-go.
