## ADDED Requirements

### Requirement: Периодический сбор состояния production VM

Система MUST каждые 15 минут собирать технические агрегаты VM, контейнеров,
PostgreSQL и backup без содержимого пользовательских данных.

#### Scenario: Плановый замер

- **WHEN** наступает очередной 15-минутный интервал
- **THEN** система сохраняет timestamped CPU/RAM/disk/I/O/container/database/backup metrics и результат collection без секретов и ПДн.

### Requirement: Суточная Telegram-сводка

Система MUST в 09:00 `Europe/Moscow` создать ровно одну сводку за предыдущий
календарный день для каждого active super-admin с active verified Telegram
subscription.

Период MUST быть полуинтервалом `[00:00, 00:00)` в `Europe/Moscow`; формулы,
sample coverage и recipient lookup MUST соответствовать
`docs/architecture/client-cabinet-r1-monitoring-telegram-contract.md`.

#### Scenario: Состав статистики авторизации

- **WHEN** строится суточная сводка
- **THEN** она содержит total accounts, новые регистрации, successful interactive logins, distinct logged-in users, failed logins и active sessions на конец суток; refresh/session restore не увеличивает login counters.

#### Scenario: Конфиденциальность отчёта

- **WHEN** сводка передаётся в Telegram
- **THEN** payload содержит только агрегаты и не содержит login, ФИО, email, IP, user ID, password, token или пользовательский контент.

#### Scenario: Повтор scheduler или Telegram failure

- **WHEN** scheduler повторно запускается для того же периода/получателя либо Telegram временно недоступен
- **THEN** delivery ledger/outbox создаёт одну логическую delivery, bounded retry не блокирует регистрацию/login, а неоднозначный внешний timeout обрабатывается как at-least-once с документированным риском одинакового повторного Telegram-сообщения.

#### Scenario: Неполные метрики

- **WHEN** собрано менее 90% ожидаемых samples либо collector/backup state неизвестен
- **THEN** значения не подменяются нулями, report помечается `ДАННЫЕ НЕПОЛНЫЕ` и показывает coverage и только имя проблемной подсистемы.

### Requirement: Критические уведомления и восстановление

Система MUST создавать alert при согласованном превышении CPU/RAM/disk,
просроченном backup, unhealthy/restarted container или всплеске failed logins и
MUST подавлять повтор одинакового active incident до recovery.

#### Scenario: Порог и нормализация

- **WHEN** CPU/RAM выше 85% в двух samples или выше 95% в одном, либо выполняется другой critical threshold
- **THEN** super-admin получает один alert, а после нормализации — одно recovery message.

#### Scenario: Всплеск неуспешных входов

- **WHEN** за 15 минут есть минимум 50 failed interactive logins либо минимум 20 и значение не менее чем втрое выше медианы соответствующего интервала предыдущих 7 дней
- **THEN** создаётся один агрегированный incident без login/email/IP/user ID, а повторы подавляются до двух нормальных интервалов и recovery.
