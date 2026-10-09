# R1 load и capacity report

Дата: 9 октября 2026 года.

## Цель

Проверить release-кандидат R1 при целевом для стартовой VM профиле: до 100
одновременно активных пользователей, 25 req/s продолжительно и 50 req/s в
коротком пике. Это не утверждение о 10 000 одновременных пользователей: 10 000 —
число зарегистрированных аккаунтов, из которых расчётно активны до 100.

## Конфигурация контейнеров

| Сервис | CPU limit | RAM limit | PID limit |
| --- | ---: | ---: | ---: |
| Caddy | 0,5 | 256 MiB | 128 |
| Next.js | 1,5 | 2 GiB | 256 |
| Go API | 1,5 | 1 GiB | 256 |
| PostgreSQL | 2 | 4 GiB | 256 |
| backup | 0,75 | 768 MiB | 128 |
| migrate | 0,5 | 512 MiB | 128 |

PostgreSQL ограничен 80 connections и `shared_buffers=1GB`. Backup и migrate не
работают постоянно одновременно с пиковой нагрузкой.

## Инструмент

`scripts/r1-load-test.mjs` выполняет browser-like login с Origin protection,
затем `/api/v1/me`, bounded список проектов и landing. Параметры задаются через
`LOAD_VUS`, `LOAD_DURATION_SECONDS`, `LOAD_TARGET_RPS`; логин и пароль не
выводятся в отчёт. Режим `LOAD_SHARED_SESSION=true` моделирует большое число
одновременно активных клиентов без искусственного срабатывания rate limit
одного логина.

## Измерения локального release-кандидата

### Peak-smoke

- 100 virtual users;
- 50 target req/s;
- 60 секунд;
- 3 001 запрос, фактически 50,02 req/s;
- error rate: 0%;
- read p95: 101,26 ms при критерии ≤500 ms;
- login p95: 82,05 ms при критерии ≤800 ms.

Снимок ресурсов во время теста:

| Сервис | CPU | RAM |
| --- | ---: | ---: |
| API | 5,60% | 128,6 MiB / 1 GiB |
| web | 0,00% в момент снимка | 55,67 MiB / 2 GiB |
| PostgreSQL | 0,59% | 87,15 MiB / 4 GiB |
| Caddy | 0,00% в момент снимка | 51,49 MiB / 256 MiB |

Предшествующий непрерывный 4-VU профиль дал 44,53 req/s, error rate 0%, read
p95 20,75 ms и login p95 161,72 ms; API/web/PostgreSQL оставались существенно
ниже memory limits.

## Вывод

Функциональные latency/error thresholds R1 для 100 активных пользователей и
пика 50 req/s подтверждены на локальном Docker release-кандидате. Ограничения
контейнеров применяются и не вызвали OOM/restart. Начальная рекомендация VM
остаётся 4 vCPU / 8 GiB / 150 GiB NVMe для до 10 000 зарегистрированных и до
100 одновременно активных пользователей.

Локальный loopback не измеряет VPS network/disk contention. Read-only SSH probe
9 октября 2026 года был отклонён сервером (`Permission denied`), поэтому CPU,
RAM и диск текущего VPS не объявлены подтверждёнными. До production go/no-go
нужно повторить 30-минутный sustained, 10-минутный peak и 60-минутный soak на
staging/VPS, снять p95 CPU/RAM/I/O, backup impact и сравнить с теми же порогами.

## Команда повторения

```sh
LOAD_BASE_URL=https://staging.example.ru \
LOAD_VUS=100 LOAD_DURATION_SECONDS=1800 LOAD_TARGET_RPS=25 \
LOAD_SHARED_SESSION=true LOAD_USERNAME='…' LOAD_PASSWORD='…' \
npm run test:load:r1
```

Credentials передаются из защищённой среды и не добавляются в shell history,
CI artifact или логи.
