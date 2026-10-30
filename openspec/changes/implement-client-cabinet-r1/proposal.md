## Why

Согласованный R1 должен превратить публичный сайт в платформу с защищённой
учётной записью и проектами, не нарушив работу лендинга и контактной формы.
Одновременно требуется эксплуатационный контроль VM и суточная статистика
регистраций/авторизаций для супер-администратора.

## Status

- Scope R1: accepted пользователем 2026-09-22.
- Monitoring and daily Telegram summary: accepted пользователем 2026-09-22.
- User auth delivery channel (email/Telegram) и Telegram login/password
  confirmation: accepted пользователем 2026-09-24; точный UX-текст согласован.
- Implementation: not started.

## From → To

- **From:** публичный Next.js-сайт, Go API контактной формы, PostgreSQL,
  системная Telegram-подписка администратора и один Docker Compose узел.
- **To:** самостоятельная регистрация с подтверждением через email или проверку
  регистрационных login/password в новом личном Telegram-чате, защищённая
  cookie-сессия, восстановление пароля, управление пользователями
  супер-администратором, список и создание проектов, включая проект без
  заказчика, плюс наблюдаемость и Telegram-сводка за календарные сутки.

## Scope

- Маршруты `/login`, `/register`, `/verify-email`, `/forgot-password`,
  `/reset-password`, `/account/projects`, `/account/projects/new`,
  `/account/users`.
- Регистрация: login, email, password/confirmation, обязательное имя,
  необязательные отчество/фамилия и расширяемая профессиональная роль.
- Выбор email/Telegram после регистрации; в Telegram пользователь открывает
  бота по ссылке и подтверждает pending account исходными login/password,
  после чего private chat связывается автоматически; повторная отправка,
  вход/выход, восстановление сессии, forgot/reset password через подтверждённый
  email или заранее связанный chat, rotation/revocation и отключение пользователя.
- Главный администратор использует существующие системные credentials как
  bootstrap super-admin, после чего проходит тот же session boundary.
- Проект создаётся обычным пользователем с ним как customer/project-admin;
  super-admin может создать и вести проект без customer и назначить его позже.
- Super-admin видит пользователей, registration/activity и отключает/
  восстанавливает доступ, но не может отключить последнего active super-admin.
- Суточная Telegram-сводка активным подписанным super-admin и критические
  alerts по VM/контейнерам/PostgreSQL/backup/auth failures.
- Совместимая PostgreSQL migration, data dictionary, OpenAPI, Cypress,
  unit/component/integration tests, backup/restore и нагрузочная приёмка.

## Non-goals

- Задачи, материалы, финансы, календари, чаты и отчёты продукта R2–R7.
- Социальная авторизация, MFA, SSO и публичное API enumeration пользователей.
- Высокая доступность из нескольких VM; R1 сохраняет один production-узел.
- Хранение пользовательских файлов на системном диске VM.
- Отправка логинов, email, IP, user ID и других персональных данных в исходящих
  Telegram-уведомлениях. Единственное исключение — login и password, которые
  сам пользователь вводит в private chat в согласованном сценарии подтверждения;
  для них действует отдельная политика удаления и запрета хранения.
- Хранение password из Telegram в БД, log, audit, outbox или состоянии диалога;
  сообщение с password должно удаляться ботом сразу после проверки.

## Compatibility and rollout

- Публичные маршруты и контактная форма сохраняют текущие контракты.
- Сначала применяется additive migration, затем новая версия API и frontend.
- Старый код должен продолжать работать с расширенной схемой во время deploy.
- До migration создаётся off-site backup; восстановление проверяется отдельно.
- Release проходит рабочая ветка → PR `develop` → PR `main` → tag `0.2.0`.

## Sources

- [`docs/releases/client-cabinet/0.2.0-tasks.md`](../../../docs/releases/client-cabinet/0.2.0-tasks.md)
- [`docs/architecture/client-cabinet-release-plan.md`](../../../docs/architecture/client-cabinet-release-plan.md)
- [`docs/architecture/future-client-cabinet-spec.md`](../../../docs/architecture/future-client-cabinet-spec.md)
- [`docs/architecture/client-cabinet-capacity-plan.md`](../../../docs/architecture/client-cabinet-capacity-plan.md)
