# CLI-adapters и capability negotiation

Core выбирает стратегию по возможностям, а не по бренду агента. Встроенные
Codex, Claude и Gemini adapters — примеры реализации одного контракта;
неизвестный CLI подключается declarative manifest без изменения state schema.

## Discovery

Используй `statectl runtime-probe --adapter auto`. Порядок выбора:

1. явный `--adapter`;
2. `EXECUTION_STATE_ADAPTER`, заданный внешним wrapper;
3. единственный доступный adapter среди разрешённых для discovery;
4. `manual` fallback при нуле или нескольких кандидатах.

Custom manifest передаётся через `--manifest`. Его ID выбирается через
`--adapter <ID>` либо участвует в `auto` только вместе с явным
`--trust-custom-adapter`. Без trust executable custom adapter даже не получает
`--version`.

Discovery разрешает executable через `PATH` и может вызвать только `--version`
с коротким timeout. Он не отправляет model request. Если найдено несколько CLI,
`auto` возвращает `manual` и список кандидатов: выбор модели, цены и permissions
нельзя делать неявно.

## Общие capabilities

| Capability | Значение |
| --- | --- |
| `fresh_context` | Новый процесс начинает чистый model context |
| `in_place_compaction` | Runtime умеет compaction в выбранной boundary |
| `machine_output` | Результат можно детерминированно разобрать |
| `session_persistence_control` | CLI умеет отключить сохранение session |
| `usage_metrics` | Adapter умеет нормализовать реальные usage metrics |

Отсутствующее или неподтверждённое поле считается `false`. Не оценивай
неизвестные token values по числу символов как реальные usage metrics.

## Встроенные adapters

- Codex использует новый `codex exec` без resume; `--ephemeral` предотвращает
  сохранение session files, stdin передаёт prompt, `--json` даёт JSONL events.
  См. [официальный Codex CLI reference](https://developers.openai.com/codex/cli/reference).
- Claude Code использует новый print-mode процесс без resume и с отключённым
  session persistence; machine output и schema включаются только поддерживаемыми
  текущей версией флагами. См. [официальный Claude Code CLI reference](https://code.claude.com/docs/en/cli-usage).
- Gemini CLI использует новый non-interactive процесс и stream-json output;
  неподтверждённые persistence/schema capabilities остаются `false`. См.
  [официальный Gemini CLI headless reference](https://geminicli.com/docs/cli/headless/).
- `manual` никогда не запускает процесс: он только оставляет prompt и packet,
  которые можно передать любому агенту.

`statectl runtime-plan` строит безопасный launch plan, но не выполняет платный
agent run. Команда требует `--id <STATE_ID>` либо `--state <STATE_PATH>` и
проверяет packet против активного `worker_lease`, текущей revision, task и
содержимого state. Packet сам по себе launch plan не создаёт. Реальный runner
должен отдельно получить обычное для среды разрешение и соблюдать её
sandbox/approval policy.

Пример после успешного `packet`:

```text
<STATECTL> runtime-plan --id auth --project-root . --adapter manual \
  --prompt worker.md --packet worker-request.json
```

## Custom manifest

Минимальный manifest:

```json
{
  "schema_version": "1.0.0",
  "id": "generic-agent",
  "executables": ["my-agent"],
  "capabilities": {
    "fresh_context": true,
    "in_place_compaction": false,
    "machine_output": false,
    "session_persistence_control": false,
    "usage_metrics": false
  },
  "launch": {
    "strategy": "fresh_process",
    "argv": ["{executable}", "run", "--non-interactive"],
    "stdin_files": ["{prompt_path}", "{packet_path}"]
  }
}
```

Ограничения:

- `launch.argv` — массив отдельных аргументов, не shell-строка;
- executable берётся только из `executables` manifest;
- разрешены только документированные placeholders как целый argv-элемент;
- manifest не может включать shell, parser hooks или permission-bypass flags;
- project root передаётся как `cwd`, если adapter не имеет отдельного
  безопасного аргумента;
- prompt transport — stdin или файл;
- custom manifest используется только после явного выбора и opt-in доверия;
  `--trust-custom-adapter` разрешает запуск указанного executable с `--version`,
  поэтому непроверенный manifest нельзя передавать с этим флагом;
- trust не запускает model request: `runtime-plan` только возвращает
  `executes:false`, argv, cwd и описанный stdin framing.

Явный custom adapter вызывается так:

```text
<STATECTL> runtime-probe --adapter generic-agent \
  --manifest adapters/generic-agent.json --trust-custom-adapter
```

Неизвестный CLI без manifest остаётся в `manual-handoff`. Это безопасная
деградация, а не ошибка универсального core.

## Добавление нового CLI

1. Создай adapter manifest с минимальным набором подтверждённых capabilities.
2. Проверь `--version` и launch plan без model request.
3. Протестируй argv, cwd и prompt transport на fake executable.
4. Выполни отдельный opt-in smoke-run с реальным аккаунтом.
5. Не меняй state schema или worker protocol ради vendor-specific output;
   нормализуй его внутри adapter/runner.
