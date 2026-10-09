# R1 monitoring, daily summary and alert runbook

Статус: implemented, 9 октября 2026 года.

## Расписание

- API выполняет первый monitoring tick после старта и затем ставит timer ровно
  до следующей границы `:00/:15/:30/:45`.
- Логический sample slot всегда округлён до 15 минут; unique key в PostgreSQL
  делает повтор после restart безопасным.
- Суточная сводка формируется в 09:00 `Europe/Moscow` за предыдущие локальные
  сутки и догоняет не более семи пропущенных периодов.
- Неверная IANA timezone останавливает API до ready. Runtime image содержит
  `tzdata`, необходимый для `Europe/Moscow`.

## Собираемые данные

Внутренний collector пишет только allowlisted aggregates:

- Go goroutines/heap;
- cgroup RAM bytes/percent и CPU percent;
- filesystem used bytes/percent;
- возраст последнего проверенного off-site backup по marker-файлу shared backup
  volume; marker создаётся только после успешных Restic upload/retention/check.

Логин, email, ФИО, IP, user/project/chat ID, raw URL, SQL, тексты сообщений,
токены и cookie в sample не входят. Docker/host availability нельзя надёжно
наблюдать из того же контейнера при полном отказе VM, поэтому Compose health и
внешний provider monitor остаются обязательным независимым каналом.

## Суточная Telegram-сводка

Сводка содержит coverage samples, CPU/RAM p95, максимальное заполнение диска,
возраст backup, общее число аккаунтов, регистрации, успешные/неуспешные входы,
уникальных вошедших и активные sessions. Недоступная метрика показывается как
`n/a`; при coverage ниже 90% или partial collector сообщение начинается с
`ДАННЫЕ НЕПОЛНЫЕ`.

Получатели разрешаются транзакционно: только активные `super_admin` с активной
Telegram binding. Unique delivery ledger и idempotency key обеспечивают одну
логическую delivery на период и получателя. Telegram не поддерживает внешний
idempotency key, поэтому после неоднозначного сетевого timeout редкий внешний
дубль остаётся документированным ограничением.

## Alerts

Пороговая таблица и state machine находятся в
`docs/architecture/client-cabinet-r1-monitoring-telegram-contract.md`.
Повторный проблемный sample обновляет peak/last_seen, но не создаёт новое
сообщение. CPU/RAM/connection требуют два последовательных превышения (либо
один критический >95%); recovery также подтверждается согласно hysteresis.
Disk, stale backup и некоторые дискретные отказы активируются немедленно.

### Alert drill

Команда использует только изолированную базу с именем `*_test`:

```sh
go test -tags=integration -count=1 \
  -run 'TestMonitoring(DailyReportIsIdempotentAcrossConcurrentSchedulersAndInactiveRecipient|AlertStateMachineEmitsOnlyTransitions)$' \
  ./internal/repository
```

Результат 9 октября 2026 года: passed. Проверено:

- 8 конкурирующих scheduler создают ровно один `report_deliveries` и один daily
  outbox item;
- первый CPU problem переводит incident в pending без сообщения;
- второй создаёт ровно один problem outbox;
- повтор problem не создаёт дубль;
- два recovery observation создают ровно один recovery outbox;
- всего по incident остаются ровно два сообщения — problem и recovery;
- отключение получателя terminalize’ит ожидающую delivery.

## Диагностика

1. Проверить `monitoring_samples` за последние 30 минут и статус collector.
2. Проверить `monitoring_incidents` для открытого `(type, resource_key)`.
3. Проверить `report_deliveries` и `notification_outbox`, не расшифровывая
   payload в журнал.
4. Для backup проверить наличие свежего `latest-offsite-success` и `restic
   check`; ручное изменение marker запрещено.
5. Если API/VM полностью недоступны, использовать независимый provider alert,
   а не ждать Telegram из остановленного приложения.

Повторный запуск scheduler безопасен; ручное удаление delivery ledger ради
повторной отправки запрещено. Для operator-approved resend создаётся отдельный
аудируемый сценарий будущего релиза.
