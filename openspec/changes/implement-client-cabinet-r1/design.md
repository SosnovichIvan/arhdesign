## Architecture

```text
Browser
  │ HTTPS + HttpOnly cookies + CSRF
  ▼
Caddy ──► Next.js account UI
  │
  └─────► Go API
            ├── auth application service ──► PostgreSQL
            ├── project application service ──► PostgreSQL
            ├── user administration ──► PostgreSQL
            ├── SMTP outbox worker ──► SMTP provider
            ├── auth notification outbox ──► Telegram Bot API/relay
            └── monitoring scheduler/outbox ──► Telegram relay
```

Система остаётся модульным монолитом: один Go binary владеет HTTP adapters,
application services, доменными инвариантами и repository interfaces. Новый
микросервис не оправдан: независимого release cadence нет, а network boundary
добавил бы отказ и распределённую согласованность.

## Contracts and ownership

| Модуль | Владеет | Публичный контракт |
| --- | --- | --- |
| Identity | user, profile, role, credential, verification/reset token, delivery choice | register, choose channel, verify, login, reset, disable/restore |
| Session | session family, rotation, revocation, CSRF | refresh, logout, authenticate request |
| Projects | project, customer membership, project-admin membership | list/create/read, assign customer |
| Audit | security/project events | append-only event and bounded aggregate queries |
| Notifications | email/Telegram outbox and delivery ledger | enqueue after commit, retry idempotently |
| Monitoring | VM samples and daily report period | collect, aggregate, alert, enqueue summary |

HTTP handler только преобразует вход/выход. Application service проверяет
actor, объект и действие, открывает транзакцию и вызывает repository. Ни один
repository не принимает user-provided tenant ID без resolved authorization
scope.

## Identity and registration

Login и email сравниваются в нормализованном виде и имеют database UNIQUE.
Оригинальное отображаемое значение хранится отдельно. Создание user, profile,
credential, verification token и audit event атомарно. После commit пользователь
выбирает email либо Telegram. Email delivery ставится в outbox идемпотентно.
Для Telegram сайт открывает публичную ссылку на бота; бот последовательно
запрашивает login/password и передаёт их Identity service только для одной
проверки. Поля credentials не отображаются и не отправляются web-клиентом:
выбор Telegram немедленно запускает приложение через bot deep link, а остающийся
web-экран содержит только повторный запуск и возврат к email. Успех атомарно
активирует pending account и связывает private chat.

Профессиональная роль — строка из расширяемого справочника, а не PostgreSQL enum.
Имя обязательно; отчество/фамилия nullable. До подтверждения регистрации через
один из каналов пользователь не получает account session. Email channel также
устанавливает `email_verified_at`; Telegram channel активирует account и chat
binding сразу после успешной проверки credentials, но email остаётся unverified
до отдельного подтверждения. Resend возвращает нейтральный ответ и инвалидирует
предыдущий active email token.

## Passwords and tokens

- Password хранится только как Argon2id hash с параметрами, версионированными в
  credential record; raw password живёт только в request memory.
- Verification/reset/refresh tokens генерируются CSPRNG и хранятся только как
  keyed hash; значение выдаётся один раз.
- Verification token single-use, lifetime 24 часа.
- Telegram confirmation использует отдельное rate-limited bot-auth состояние с
  TTL 5 минут между вводом login и password. Login может храниться только
  кратковременно в зашифрованном/процессном состоянии; password не сохраняется.
- Password reset token single-use, lifetime 30 минут; успешный reset отзывает
  все sessions пользователя.
- Access session cookie: HttpOnly, Secure production, SameSite=Strict, Path=/,
  idle timeout 30 минут и absolute lifetime 24 часа. Refresh family:
  single-use rotation, inactivity 30 дней, absolute lifetime 90 дней; reuse
  отзывает всю family.
- State-changing endpoint проверяет Origin и synchronizer CSRF token.
- Login/reset/resend имеют rate limit по анонимному HMAC marker и account key;
  ответ не подтверждает существование email.
- Reset через Telegram разрешён только для chat binding, созданного до запроса
  восстановления. Отсутствующий binding и неизвестный account дают одинаковый
  публичный ответ.
- Telegram password message удаляется через Bot API immediately/best effort при
  любом результате; raw login/password не попадают в logs, audit или outbox.
  Удаление сообщения не устраняет передачу данных инфраструктуре Telegram — это
  принятый владельцем продукта residual privacy/security risk.

Точные password/token/session/CSRF параметры, production cookie boundary и
обязательная test matrix зафиксированы в
`docs/architecture/client-cabinet-r1-auth-security-adr.md`.

## Bootstrap super-admin

Существующие `ADMIN_USERNAME`/`ADMIN_PASSWORD` используются только для
одноразового bootstrap: при первом успешном входе в транзакции создаётся либо
связывается database user с global role `super_admin`; password сразу
перехешируется общей password policy. Env password после завершённого bootstrap
не является постоянным обходным входом. Повторная инициализация идемпотентна.

Последний active super-admin не может отключить себя или быть отключён другим
пользователем. Disable отзывает все sessions в той же транзакции.

## Project creation

Обычный authenticated user создаёт project в одной транзакции вместе с active
membership `customer` и `project_admin`. Customer обязателен для такого actor.

Super-admin может создать project с `customer_user_id = NULL`; creator и audit
сохраняются, но фиктивная customer membership не создаётся. Позднее назначение
customer создаёт единственную active customer membership и не переписывает
creator/audit/history. Попытка назначить второго active customer конфликтует с
database constraint.

## User administration

Список пользователей доступен только super-admin, ограничен pagination и
stable sort. Activity — время последней успешной интерактивной авторизации, а
не refresh. Поиск не возвращает password/token hashes. Disable/restore имеет
optimistic version, audit и idempotent повтор.

## Monitoring and daily summary

VM metrics собираются каждые 15 минут. Ежедневный report period — предыдущий
календарный день `00:00:00–23:59:59 Europe/Moscow`; отправка в 09:00. Auth
aggregate содержит total accounts, new registrations, successful interactive
logins, distinct users with successful login, failed logins и active sessions
на конец периода. Refresh/session restore не является login.

Получатели вычисляются в момент отправки: active database user с global role
`super_admin` и active verified private Telegram subscription. Payload содержит
только агрегаты. UNIQUE `(report_type, period_start, period_end, recipient_id)`
и distributed advisory lock исключают вторую логическую delivery после restart.
Telegram не принимает application idempotency key, поэтому неоднозначный
внешний timeout остаётся at-least-once boundary с редким риском одинакового
повторного сообщения.

Summary/alert записывается в outbox до внешнего вызова. Telegram timeout не
блокирует auth. CPU/RAM >85% в двух samples или >95% в одном, disk >80%, backup
age >26h, unhealthy/restarted container и threshold failed logins создают alert.
Одинаковый active incident подавляется до recovery event.

## Data and migration order

1. Add reference/global identity tables and audit/outbox without changing
   current contact tables.
2. Add users/profiles/credentials/tokens/sessions and constraints.
3. Add projects/memberships and partial uniqueness for one active customer.
4. Add monitoring samples/report deliveries and supporting indexes.
5. Seed professional roles; super-admin is not pre-seeded from plaintext env.
6. Deploy API, run bootstrap on first authenticated request, deploy UI.

Migration проверяется на пустой PostgreSQL 17 и на копии текущей schema с
данными. Применённая migration не редактируется. Down не обещает вернуть
удалённые security tokens; recovery выполняется restore в новый instance.

## External failures

SMTP/Telegram имеют explicit timeout, bounded exponential retry и terminal
failure state. User-visible registration succeeds после database commit даже
при временной недоступности SMTP, но UI предлагает resend. Password reset
возвращает generic response. Failed outbox наблюдаем и может быть безопасно
replayed по idempotency key.

## Alternatives

| Option | Decision |
| --- | --- |
| JWT в localStorage | Rejected: XSS раскрывает token и усложняет отзыв. |
| Stateless JWT без session table | Rejected: disable/reuse detection требует немедленного server-side revoke. |
| PostgreSQL enum для профессиональных ролей | Rejected: роли должны расширяться без DDL. |
| Отдельный auth microservice | Rejected для R1: лишняя сеть и operational cost без независимого владельца. |
| Cron shell на VM для Telegram report | Rejected: не знает auth semantics и сложно обеспечить idempotency/PII policy. |
| Хранить агрегированный баланс пользователей | Rejected: суточные auth counts строятся из audit events и delivery ledger. |

## Verification gates

- Все protected endpoints имеют positive и negative object authorization tests.
- PostgreSQL integration использует PostgreSQL 17, включая uniqueness/races.
- Auth/monitoring domain code достигает project coverage gate 90%.
- Cypress проверяет register/verify/login/reset/projects/users и чужой URL.
- Load test и restore drill выполняются до tag `0.2.0`.
