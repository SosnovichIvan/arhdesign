# R1: каталог запросов и индексов

Статус: проверено локально на PostgreSQL 17.11. Тестовая схема создавалась отдельно
в локальной БД, наполнялась синтетическими данными и была удалена после
`EXPLAIN (ANALYZE, BUFFERS)`. Запросы на production не выполнялись.

## Условия измерения

- PostgreSQL `17-alpine` из `infra/docker-compose.yml`;
- 10 000 users: 95% active, 5% disabled;
- 10 000 verification tokens;
- 30 000 sessions: три на пользователя, 20% revoked;
- 20 000 projects и 30 000 active memberships;
- отдельный ledger fixture: 100 000 audit events и 365 daily deliveries;
- после наполнения выполнен `ANALYZE` всех таблиц;
- планы warm-cache, поэтому миллисекунды не являются production SLO;
- synthetic UUID и равномерное распределение не моделируют skew реальной БД.

Login не содержит `@` и пробелы. Email всегда содержит `@`; namespaces поэтому
не пересекаются, и backend выбирает один из двух запросов вместо `OR`.

## Каталог

### Q1 — пользователь по login

Частота: каждый интерактивный вход и bootstrap super-admin. Результат: `0..1`.

```sql
SELECT id, login_normalized, email_normalized, status
FROM users
WHERE login_normalized = $1;
```

Индекс:

```sql
CREATE UNIQUE INDEX users_login_normalized_uidx
    ON users (login_normalized);
```

Проверка: `Index Scan`, estimated/actual `1/1`, buffers hit `3`, execution
`0.015 ms`. Индекс одновременно обеспечивает конкурентную уникальность.

### Q2 — пользователь по email

Частота: login по email, register/forgot/resend. Результат: `0..1`.

```sql
SELECT id, login_normalized, email_normalized, status
FROM users
WHERE email_normalized = $1;
```

Индекс:

```sql
CREATE UNIQUE INDEX users_email_normalized_uidx
    ON users (email_normalized);
```

Проверка: `Index Scan`, estimated/actual `1/1`, buffers hit `3`, execution
`0.015 ms`. Generic HTTP-ответ не зависит от найденной строки.

### Q3 — активный verification/reset token

Частота: переход по одноразовой ссылке; результат: `0..1`.

```sql
SELECT id, user_id, expires_at
FROM email_verification_tokens
WHERE token_hash = $1
  AND consumed_at IS NULL
  AND expires_at > now();
```

`token_hash` имеет `UNIQUE`; expiry/consumed проверяются фильтром после одного
index hit. Проверка: `Index Scan`, estimated/actual `1/1`, buffers hit `3`,
execution `0.009 ms`. Аналогичный contract применяется к reset token.

Отдельный partial index нужен для bounded cleanup, а не lookup:

```sql
CREATE INDEX email_verification_tokens_expiry_idx
    ON email_verification_tokens (expires_at)
    WHERE consumed_at IS NULL;
```

Cleanup выбирает не более 1 000 строк за batch по `expires_at`, фиксирует
checkpoint и не удерживает одну транзакцию на весь retention backlog.

### Q4 — refresh session

Частота: один запрос на controlled refresh; результат: `0..1`.

```sql
SELECT id, user_id, family_id, idle_expires_at, absolute_expires_at
FROM sessions
WHERE refresh_token_hash = $1
  AND revoked_at IS NULL
  AND idle_expires_at > now()
  AND absolute_expires_at > now();
```

`refresh_token_hash UNIQUE`: `Index Scan`, estimated/actual `1/1`, buffers hit
`3`, execution `0.009 ms`. Для revoke/count пользователя:

```sql
CREATE INDEX sessions_active_user_idx
    ON sessions (user_id, absolute_expires_at)
    WHERE revoked_at IS NULL;
```

`UPDATE ... WHERE user_id=$1 AND revoked_at IS NULL` проверяет affected rows и
выполняется в транзакции disable/reset. Rotation использует conditional update
по `id` и `revoked_at IS NULL`, чтобы повтор токена не создавал вторую session.

Security ADR R1 добавляет отдельный unique lookup по `access_token_hash` и
`hash_key_version`. Его план должен быть повторно измерен на целевой additive
migration в R1-DATA-004; текущий измеренный fixture содержал только refresh hash,
поэтому результат для access lookup здесь намеренно не выдуман.

### Q5 — проекты доступные actor

Частота: открытие списка/пагинация. Результат: максимум `25`; порядок устойчивый.

```sql
SELECT p.id, p.name, p.status, p.created_at
FROM projects p
WHERE EXISTS (
    SELECT 1
    FROM project_memberships pm
    WHERE pm.project_id = p.id
      AND pm.user_id = $1
      AND pm.revoked_at IS NULL
)
  AND p.status = $2
  AND (p.created_at, p.id) < ($3, $4)
ORDER BY p.created_at DESC, p.id DESC
LIMIT 25;
```

Для первой страницы cursor predicate опускается. Индексы:

```sql
CREATE UNIQUE INDEX project_memberships_one_active_role_per_user_uidx
    ON project_memberships (project_id, user_id, project_role)
    WHERE revoked_at IS NULL;

CREATE INDEX project_memberships_active_user_projects_idx
    ON project_memberships (user_id, project_id)
    WHERE revoked_at IS NULL;
```

`EXISTS` обязателен: один user может иметь несколько активных ролей в одном
проекте, а список не должен дублировать карточку. На финальной DDL actor имел
`502` membership rows для `501` разных проектов: `Bitmap Index Scan` по
`project_memberships_active_user_projects_idx` → unique project IDs → `501`
`projects_pkey Index Scan` → top-N heapsort `27 kB`; buffers hit `1 521`,
execution `0.625 ms`, возвращено `25` строк без дублей. Большое расхождение
estimate `2` против actual `502` вызвано намеренно skewed fixture; при реальном
skew следует поднять statistics target или пересмотреть denormalized sort key,
но текущий warm-cache план остаётся bounded и быстрым для R1.

### Q6 — страница пользователей для super-admin

Частота: административный экран; результат: максимум `50`; cursor pagination.

```sql
SELECT id, login_normalized, email_normalized, status, last_login_at, created_at
FROM users
WHERE status = $1
  AND (created_at, id) < ($2, $3)
ORDER BY created_at DESC, id DESC
LIMIT 50;
```

Индекс:

```sql
CREATE INDEX users_status_created_idx
    ON users (status, created_at DESC, id DESC);
```

План первой страницы: `Index Scan`, limit `50`, estimated qualifying rows
`9 500`, actual returned `50`, buffers hit `43`, execution `0.023 ms`.
`last_login_at` читается из users и отражает только успешный интерактивный
login, а не refresh.

Поиск на этом экране ограничен exact login/email через Q1/Q2. Поиск по ФИО
перенесён за пределы R1: без подтверждённого UX и распределения данных trigram
index добавлять нельзя.

### Q7 — суточные auth aggregate из audit

Частота: один отчёт в сутки плюс операторский backfill; результат: одна строка
агрегата, raw events наружу не возвращаются.

```sql
SELECT count(*)
FROM audit_events
WHERE event_type = $1
  AND result = $2
  AND occurred_at >= $3
  AND occurred_at < $4;
```

Индекс:

```sql
CREATE INDEX audit_events_type_result_occurred_at_idx
    ON audit_events (event_type, result, occurred_at DESC);
```

На 100 000 равномерных events запрос одного дня использовал `Bitmap Index
Scan` → `Bitmap Heap Scan`, estimated/actual `7 978/8 160`, heap blocks `195`,
buffers hit `284`, execution `0.858 ms`. Порядок equality/equality/range
соответствует запросу; цена — один btree write на каждый audit event.

### Q8 — одна логическая daily delivery

Создание выполняется конкурентно:

```sql
INSERT INTO report_deliveries (
    report_type, period_start, period_end, recipient_id, aggregate_payload
)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (report_type, period_start, period_end, recipient_id) DO NOTHING;
```

`UNIQUE (report_type, period_start, period_end, recipient_id)` является
correctness boundary. Integration test с 20 конкурентными writers получил
ровно одного winner и одну строку. На 365 deliveries точечное чтение по
recipient/period выбрало `report_deliveries_recipient_idx`, buffers hit `5`,
execution `0.018 ms`; unique индекс всё равно необходим для гонки scheduler.

### Q9 — claim очередного email outbox

Частота: до одного claim на итерацию email worker; результат `0..1`. Claim и
перевод в `delivering` выполняются одним statement, а `SKIP LOCKED` позволяет
нескольким экземплярам не отправлять одну строку одновременно.

```sql
WITH due AS (
    SELECT id
    FROM notification_outbox
    WHERE channel = 'email'
      AND state IN ('pending', 'retry')
      AND available_at <= $1
    ORDER BY available_at, created_at, id
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE notification_outbox outbox
SET state = 'delivering', attempts = attempts + 1, locked_at = $1
FROM due
WHERE outbox.id = due.id
RETURNING outbox.id;
```

Индекс из migration `007`:

```sql
CREATE INDEX notification_outbox_channel_due_idx
    ON notification_outbox (channel, available_at, created_at, id)
    WHERE state IN ('pending', 'retry');
```

`channel` — equality, `available_at` — range, следующие колонки поддерживают
стабильный порядок. Общий индекс без `channel` сохранён для будущего
мультиканального dispatcher; цена email-индекса — дополнительный btree write
только пока строка находится в `pending/retry`.

На 100 000 outbox rows (90% email, 10% Telegram, все due) PostgreSQL 17.11
выбрал `Index Scan using notification_outbox_channel_due_idx`, вернул одну
строку при estimated `89 893`, buffers hit `4` внутри scan; весь atomic
claim/update занял `0.234 ms` при warm cache. Большое число qualifying rows
ожидаемо для worst-case backlog, но `LIMIT 1` остановил scan после первой строки.

Worker перед claim переводит зависшие более пяти минут `delivering` строки в
`retry`. Migration `008` добавляет для bounded recovery partial index
`(channel, locked_at, id) WHERE state='delivering'`. Доставка остаётся
at-least-once: авария после принятия SMTP-сервером, но до `delivered` может дать
повторное письмо; потеря письма из-за навсегда зависшей строки исключается.

## Цена индексов и контроль

### Q — персональные настройки интерфейса и проекта

Тема читается по PK `user_settings(user_id)` и записывается idempotent
`INSERT ... ON CONFLICT (user_id) DO UPDATE`. Период читается и записывается по
составному PK `project_user_settings(project_id,user_id)` после проверки active
membership в том же statement snapshot. Значение по умолчанию возвращается без
создания строки. Отдельный индекс не добавляется: оба access path покрыты PK, а
проверка участия использует существующий partial index
`project_memberships_active_user_projects_idx(user_id,project_id)`.

Ожидаемая частота — один read темы при инициализации сессии UI, один read периода
при открытии проекта и редкие writes по явному действию пользователя. Размер
результата всегда одна строка; N+1 отсутствует.

| Индекс | Чтение | Цена записи/хранения |
| --- | --- | --- |
| login/email unique | login, register conflict | два btree update на user mutation |
| token hash unique | single-use lookup | один btree на созданный token |
| token expiry partial | retention batch | только active token; удаляется вместе со строкой |
| session hash unique | refresh/reuse | один btree на rotation |
| active sessions by user | revoke/count | запись только active session; churn при revoke |
| active membership unique | race-safe membership | update индекса при join/revoke |
| user settings PK | theme read/upsert | один btree write при изменении темы |
| project user settings PK | period read/upsert | один btree write при изменении периода |
| memberships by user | project list | второй membership btree ради обратного направления |
| users status/created | admin page | update при disable/restore |
| audit type/result/time | daily aggregate и security review | один btree write на event; retention удаляет batch |
| outbox due partial | bounded worker claim | индексируется только pending/retry; churn при state transition |
| report logical unique | scheduler race/idempotency | один btree write на recipient/period |
| report recipient/time | история доставки | дополнительный btree на report row |

Перед PR migration планы повторяются на точной DDL и fixture integration tests.
Проверяются actual rows/loops/buffers и отсутствие N+1 в repository tests. Seq
scan на маленькой таблице не считается дефектом сам по себе.

## Команда проверки

Тест выполнен через локальный контейнер `postgres:17-alpine` командой `psql`
с `ON_ERROR_STOP=1`; временная схема `r1_plan_test` удалена в конце сценария.
Исходный одноразовый fixture не является migration и в репозиторий не включён.
