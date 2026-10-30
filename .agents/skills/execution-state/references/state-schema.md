# Compact state schema 1.0.0

`statectl` — единственная публичная точка управления. Все команды печатают одну
короткую JSON-строку; packet записывается только в файл.

## Размещение

```text
.execution-state/<id>/
├── state.json
├── tasks.json       # только standalone; не входит в worker packet
└── context-map.json # опциональный компактный навигационный индекс
```

В OpenSpec `tasks.json` отсутствует: task ledger уже находится в `tasks.md`.
Prompt templates и runtime adapter manifests не копируются в state.

## `state.json`

```json
{
  "schema_version": "1.0.0",
  "id": "add-rate-limit",
  "revision": 4,
  "source": {
    "kind": "openspec",
    "change": "add-rate-limit",
    "tasks_path": "openspec/changes/add-rate-limit/tasks.md"
  },
  "execution": {
    "profile": "reset",
    "adapter": "codex",
    "capabilities": {
      "fresh_context": true,
      "in_place_compaction": false,
      "machine_output": true,
      "session_persistence_control": true,
      "usage_metrics": true
    }
  },
  "implementation_ref": "/backend-api",
  "goal": "Завершить проверяемый OpenSpec change",
  "status": "in_progress",
  "constraints": ["Не менять публичный API"],
  "active_task": {
    "id": "2.1",
    "title": "Реализовать middleware",
    "status": "in_progress",
    "done_when": ["Реализовать middleware"],
    "evidence": [],
    "checks": [],
    "artifacts": ["src/middleware/rate_limit.py"],
    "kind": "implementation",
    "cohesion_key": "rate-limit",
    "affected_areas": ["http", "security"],
    "reads": ["src/http/router.py"],
    "writes": ["src/middleware/rate_limit.py"],
    "contracts": ["HTTP 429 сохраняет общий error envelope"],
    "regression_checks": ["rate-limit-black-box", "error-envelope"],
    "requires_bridge": true
  },
  "next_action": "Run the targeted regression checks",
  "observation": "Implementation changed; targeted checks are pending",
  "blocker": null,
  "artifacts": ["src/middleware/rate_limit.py"],
  "checkpoint": {
    "ready": false,
    "reason": "new observation",
    "revision": 4
  },
  "quality": {
    "invariants": ["Публичный error envelope одинаков для всех handlers"],
    "completed_chunks": 7,
    "review_interval": 8,
    "review_required": false,
    "review_reasons": [],
    "last_review": null,
    "pending_bridge": false,
    "next_handoff": "reset"
  },
  "worker_lease": null
}
```

`implementation_ref` — непрозрачная ссылка в синтаксисе текущего CLI либо
`base-agent`; core не требует конкретного префикса. Adapter/capabilities —
ненормативный runtime snapshot и не входят в worker packet. При смене CLI их
нужно определить заново.

Статусы state: `planned`, `in_progress`, `blocked`, `complete`. Active task
имеет статус `in_progress` или `blocked`. `begin`, `observe`, `complete`, `block`
и `checkpoint` требуют `--expected-revision`; для безопасного handoff всегда
передавай его и в `packet`. Конфликт не меняет файлы.

`quality.invariants` действуют на все chunks. `review_interval` инициирует
архитектурную проверку после N завершений. `pending_bridge` разрешает следующий
packet только для задачи `kind=integration`. `next_handoff` — переносимая
рекомендация `continue`, `reset` или `checkpoint`, а не vendor-команда.
`architecture-review` принимает typed JSON: `protocol`, `verdict`, `summary`,
`blockers`, `planned_gaps`, `recommendations` и `checks`. В
`quality.last_review` детерминированно сохраняется типизированная сводка не
более 1 KiB; полный review остаётся во внешнем артефакте и не переносится между
workers. `blocked` создаёт recovery chunk для standalone либо переводит
OpenSpec state в `blocked`; recovery всегда требует повторного review.

## Язык сохраняемых данных

Для компактного переносимого state используй следующую политику без добавления
отдельных полей в schema:

```text
storage_language: en
source_content_language: preserve
```

Краткий технический английский обязателен для текста, который coordinator или
worker создаёт во время выполнения: `next_action`, `observation`, `blocker`,
`checkpoint.reason`, summaries/evidence, check summaries,
`quality.last_review`, а также `purpose` и другие описания context map.

Дословный или нормативный текст сохраняй на языке источника. Это относится к
импортированным `goal`, task title, `done_when`, constraints, contracts,
invariants и source refs. Если такое поле coordinator формулирует сам, он может
сразу записать его на английском; не переводи уже заданную формулировку ради
единообразия. Идентификаторы, пути, symbols, команды и код не переводятся.

`statectl` проверяет UTF-8 и byte limits, но намеренно не определяет язык:
эвристика ошибалась бы на именах API, путях и смешанном нормативном тексте.
Политику выполняют coordinator и worker. Не вводи ASCII-only validation —
технический английский может законно содержать Unicode identifiers и цитаты.
Когда явный `next_action` не передан, controller сам сохраняет компактное
`Execute task <id>`, а не копирует потенциально длинный source title.

## Worker lease и revision

`packet` — не read-only export. При успешном вызове helper создаёт новый
packet-файл, повышает state revision на 1 и записывает:

```json
{
  "worker_lease": {
    "run_id": "auth-worker-1",
    "task_id": "1",
    "based_on_revision": 1
  }
}
```

`based_on_revision` packet равен уже повышенной revision и должен без изменений
вернуться в worker result. Пока lease активен, второй `packet`, `begin` и
`checkpoint` отклоняются. Результат worker принимается через `observe`,
`complete` или `block` только с совпадающими `--run-id` и
`--expected-revision`; успешная операция снимает lease и снова повышает
revision.

## Standalone task index

```json
{
  "schema_version": "1.0.0",
  "tasks": [
    {
      "id": "auth-token",
      "title": "Реализовать token validation",
      "status": "pending",
      "done_when": ["Целевая проверка проходит"],
      "kind": "implementation",
      "cohesion_key": "auth",
      "affected_areas": ["auth"],
      "reads": ["internal/auth/service.go"],
      "writes": ["internal/auth/token.go"],
      "contracts": ["expired token отклоняется"],
      "regression_checks": ["token-black-box"],
      "requires_bridge": false
    }
  ]
}
```

Завершённые summaries/evidence могут оставаться здесь для аудита, но
`statectl packet` передаёт только активную задачу.

## Основные команды

`<STATECTL>` означает `statectl.py`, запущенный доступным Python 3 interpreter.

```text
<STATECTL> route --source standalone --expected-turns 4

<STATECTL> init --id auth --project-root . --source standalone \
  --profile lite --implementation-ref base-agent --goal "Готовый сервис" \
  --task-id 1 --task-title "Реализовать сервис" --done-when "Проверка проходит"

<STATECTL> init --id change-id --project-root . --source openspec \
  --profile reset --adapter <RESET_CAPABLE_ADAPTER> \
  --implementation-ref /backend-api --goal "Apply change" \
  --change change-id --tasks-path openspec/changes/change-id/tasks.md

<STATECTL> packet --id auth --project-root . --expected-revision 0 \
  --run-id auth-worker-1 --output worker-request.json

<STATECTL> runtime-plan --id auth --project-root . --adapter manual \
  --prompt <CONCRETE_WORKER_PROMPT> --packet worker-request.json

<STATECTL> complete --id auth --project-root . --expected-revision 1 \
  --run-id auth-worker-1 --summary "Chunk complete" \
  --check-json '{"id":"token-black-box","status":"passed","summary":"pass"}'

<STATECTL> context-map-update --id auth --project-root . \
  --expected-revision 2 \
  --entry-json '{"path":"internal/auth/token.go","purpose":"token lifecycle","areas":["auth"],"symbols":["Issuer","Validate"]}'

<STATECTL> architecture-review --id auth --project-root . \
  --expected-revision 2 \
  --review-json '{"protocol":"execution-state.review/1.0.0","verdict":"passed","summary":"Module boundaries remain intact","blockers":[],"planned_gaps":[],"recommendations":[],"checks":[{"id":"architecture-contracts","status":"passed","summary":"Dependency graph and wiring verified"}]}'

<STATECTL> validate --id auth --project-root .
```

State с другой `schema_version` отклоняется без изменения. Заверши его
совместимым release либо создай новый state текущей версии; встроенной миграции
между версиями нет.

`runtime-plan` всегда требует state binding: укажи `--id` вместе с
`--project-root` либо `--state` вместе с соответствующим `--project-root`. Он
сверяет packet с активным lease, revision и содержимым state. Для custom adapter
дополнительно нужны `--adapter <ID>`, `--manifest <PATH>` и
`--trust-custom-adapter`; trust разрешает его executable только для version
probe, а `runtime-plan` agent не запускает.

Для нескольких standalone-задач передай `--tasks-file` с объектом
`{"tasks":[...]}` или повторяй `--task-json`. Не печатай содержимое task index
и packet в model context без необходимости.

Structured check имеет поля `id`, `status: passed|failed`, `summary`. Если ID
перечислен в `regression_checks`, `complete` требует именно passed check с этим
ID. Любой переданный failed check блокирует завершение.

`context-map.json` хранит до 64 записей `{path,purpose,areas,symbols,
updated_revision}`. В packet попадают не более 16 записей, выбранных по
`reads`, `writes`, artifacts и пересечению `affected_areas`. Файл не содержит
исходный код, требования или историю.

## Лимиты и запрещённые данные

- `state.json` — 8 KiB;
- packet — 12 KiB;
- observation/blocker — 2 KiB;
- evidence текущего chunk — 4 KiB суммарно;
- raw logs, transcript, messages, history и reasoning запрещены;
- секреты и персональные данные не должны попадать ни в одно поле.

Профиль `reset` валиден только с подтверждённой `fresh_context` либо
`in_place_compaction`. `lite` не утверждает, что текущая история очищена.
