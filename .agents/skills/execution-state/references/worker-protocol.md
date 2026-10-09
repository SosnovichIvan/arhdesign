# Worker protocol 1.0.0

Coordinator — единственный писатель state и task ledger. Worker получает один
bounded semantic chunk, изменяет только разрешённые артефакты и возвращает
результат, связанный с revision.

## Request

```json
{
  "protocol": "execution-state.worker/1.0.0",
  "packet_version": "1.0.0",
  "run_id": "generated-id",
  "state_id": "add-rate-limit",
  "based_on_revision": 8,
  "language_policy": {
    "operational": "en",
    "source_content": "preserve"
  },
  "source": {"kind": "standalone"},
  "goal": "Завершить проверяемый результат",
  "implementation_ref": "/backend-api",
  "constraints": ["Не менять публичный API"],
  "task": {
    "id": "2.1",
    "title": "Реализовать middleware",
    "done_when": ["Целевая проверка проходит"],
    "source_refs": ["user-prompt"],
    "working_files": ["src/middleware/rate_limit.py"],
    "kind": "implementation",
    "cohesion_key": "rate-limit",
    "affected_areas": ["http", "security"],
    "reads": ["src/http/router.py"],
    "writes": ["src/middleware/rate_limit.py"],
    "contracts": ["HTTP 429 сохраняет error envelope"],
    "regression_checks": ["rate-limit-black-box", "error-envelope"]
  },
  "next_action": "Implement middleware and run the declared checks",
  "last_observation": "The dependent contract is already implemented",
  "quality": {
    "invariants": ["Публичный error envelope одинаков для всех handlers"],
    "next_handoff": "reset"
  },
  "context": [
    {
      "path": "src/http/router.py",
      "purpose": "HTTP routing and middleware wiring",
      "areas": ["http"],
      "symbols": ["build_router"],
      "updated_revision": 6
    }
  ],
  "limits": {"max_turns": 8, "max_result_bytes": 8192}
}
```

`packet` повышает revision, создаёт lease и записывает post-packet revision в
`based_on_revision`. Второй packet запрещён до приёмки результата.

`implementation_ref` — непрозрачная CLI-native ссылка. `context` — только
навигационный индекс; worker открывает файл перед изменением и не считает
`purpose` источником требований. Global invariants, task contracts и
`done_when` обязательны одновременно.

Packet не содержит transcript, завершённые задачи, скрытые рассуждения, полные
логи и vendor/runtime metadata.

`language_policy` является обязательным переносимым контрактом. Значение
`operational: en` требует краткий технический английский для создаваемых worker
операционных полей. Значение `source_content: preserve` запрещает переводить
нормативные поля, импортированные из запроса или task source (`goal`, title,
`done_when`, constraints, contracts, invariants), и требует сохранить их точную
формулировку.

## Result

```json
{
  "protocol": "execution-state.result/1.0.0",
  "run_id": "generated-id",
  "based_on_revision": 8,
  "task_id": "2.1",
  "status": "complete",
  "summary": "Middleware implemented",
  "artifacts": [
    {"path": "src/middleware/rate_limit.py", "purpose": "Rate limiting"}
  ],
  "checks": [
    {"id": "rate-limit-black-box", "status": "passed", "summary": "429 verified"},
    {"id": "error-envelope", "status": "passed", "summary": "envelope unchanged"}
  ],
  "context_updates": [
    {"path": "src/middleware/rate_limit.py", "purpose": "Rate limiting", "areas": ["http", "security"], "symbols": ["RateLimit"]}
  ],
  "blockers": [],
  "next_action": null
}
```

Не возвращай chain-of-thought. Если CLI не поддерживает structured output,
последний ответ содержит один JSON между маркерами:

```text
---EXECUTION_STATE_RESULT_1_0_0---
{...}
---END_EXECUTION_STATE_RESULT_1_0_0---
```

## Приём

Coordinator проверяет protocol, lease, revision, task ID, изменённые файлы,
`done_when`, contracts и точное покрытие `regression_checks`. Затем передаёт
каждый check в controller:

```text
<STATECTL> complete --id add-rate-limit --project-root . \
  --expected-revision 8 --run-id generated-id \
  --summary "Middleware implemented" \
  --check-json '{"id":"rate-limit-black-box","status":"passed","summary":"429 verified"}' \
  --check-json '{"id":"error-envelope","status":"passed","summary":"envelope unchanged"}'
```

`context_updates` принимаются отдельными вызовами `context-map-update` после
снятия lease. Для частичного результата используется `observe`, для
проверенного блокера — `block`. Несовпадение lease/revision не меняет state.

Worker возвращает созданные им `summary`, check summaries, artifact/context
`purpose`, blockers и `next_action` на кратком техническом английском. Он не
переводит цитируемые требования, identifiers, paths, symbols, команды и код.

При невалидном envelope разрешена одна попытка восстановить только формат;
затем создаётся recovery chunk или blocker. Один state допускает один активный
worker lease. Параллельность возможна лишь через независимые state ID с
непересекающимися файлами и контрактами.
