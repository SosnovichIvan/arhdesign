# execution-state

`execution-state` — explicit-only skill для долгих агентных задач. Он хранит
компактное проверяемое состояние между semantic chunks и может передавать
работу в новый context без transcript.

Текущая версия: `1.0.0`. Канонические версии runtime contracts хранятся в
`release.json` и доступны через `statectl version`. Публичные state, task,
context-map и adapter schemas, packet format и worker/result/review/stdin
protocols также зафиксированы как `1.0.0`.

State schema `1.0.0` включает quality contracts: глобальные инварианты,
именованные regression checks, компактную context map, integration bridge,
периодический architecture review и adaptive handoff по cohesion key.

Core не зависит от Codex, Claude, Gemini или модели. Agent CLI подключается
через capability-based adapter; OpenSpec является отдельным источником задач и
не влияет на выбор runtime.

## Маршрутизация

```text
явный вызов execution-state
├── короткая связная задача → passthrough, state не создаётся
├── reset недоступен        → lite checkpoint
└── длинная задача          → chunk → checkpoint → fresh/compact/manual
```

Источник требований (`standalone` или `openspec`) и профиль выполнения (`lite`
или `reset`) хранятся независимо. Для OpenSpec `tasks.md` остаётся единственным
task ledger; execution-state не создаёт task JSON.

## Совместимость CLI

| Runtime | Lite | Fresh context | Machine output | Особенность |
| --- | ---: | ---: | ---: | --- |
| Codex CLI | Да | Новый `codex exec --ephemeral` | JSONL | Встроенный adapter |
| Claude Code | Да | Новый `claude -p --no-session-persistence` | JSON stream | Встроенный adapter |
| Gemini CLI | Да | Новый non-interactive process | JSON stream | Persistence/schema capabilities ограничены |
| Другой CLI | Да | Через manifest | По manifest | Неизвестное безопасно становится `manual` |
| Любой UI/CLI без subprocess | Да | Manual handoff | Не требуется | Packet передаётся пользователем/runtime |

Auto-discovery не выполняет model request и не выбирает молча между несколькими
установленными CLI. Vendor-specific argv живёт только в adapters; state и worker
protocol остаются одинаковыми.

Skill не может стереть контекст уже работающей модели. В профиле `reset`
`statectl` создаёт ограниченный packet и проверяемый launch plan, а доверенный
внешний runner открывает новую CLI-сессию и передаёт ей конкретизированный
worker prompt. Resume/fork старой сессии строгим reset не считаются.

## Требования

| Компонент | Когда нужен | Для чего |
| --- | --- | --- |
| Полная папка `execution-state/` | Всегда | `SKILL.md` использует относительные `scripts/` и `references/` |
| Python 3.9+ | Профили `lite` и `reset` | Запуск `scripts/statectl.py`; используются только модули стандартной библиотеки |
| Права чтения и записи в проекте | Профили `lite` и `reset` | Создание `.execution-state/`, checkpoint и worker packet |
| Один agent CLI или совместимый custom adapter | Автоматический `reset` | Создание нового model context |
| Аккаунт, авторизация и сеть выбранного CLI | Реальный fresh worker | Выполнение model request |
| Доверенный host-agent или внешний runner | Автоматический `reset` | Проверка и исполнение launch plan, который возвращает `statectl` |
| `tasks.md` с checkbox и стабильными ID | Только `source=openspec` | Источник задач и статусов OpenSpec |

Внешние Python-пакеты, виртуальное окружение и OpenSpec CLI не требуются.
Node.js может требоваться конкретному agent CLI, но не core. Git нужен только
для клонирования, а Go — только для воспроизведения benchmark.

Автоматический `reset` состоит из двух частей. `statectl runtime-plan` проверяет
state/lease и возвращает `executes:false`, безопасный argv и stdin framing.
Фактический запуск делает host-agent или доверенный runner в рамках обычных
permissions. Без него используй `manual-handoff` либо `lite`; экономия токенов
внутри текущей сессии тогда не гарантируется.

Windows-код имеет portable fallback для блокировок, но CI на Windows пока не
настроен. Подтверждённая среда текущей версии — macOS с Python 3.9; Linux
покрывается POSIX-механизмами, но также должен проверяться в CI перед
production-развёртыванием.

## Установка

Клонируй репозиторий и не отделяй `SKILL.md` от остальных файлов:

```bash
git clone https://github.com/SosnovichIvan/agent-skills-lab.git
cd agent-skills-lab
python3 -c 'import sys; assert sys.version_info >= (3, 9), sys.version'
python3 skills/execution-state/scripts/statectl.py --help
```

Затем подключи каталог `skills/execution-state` в native skills-directory
целевого агента. Можно скопировать папку или создать symlink:

| Агент | User scope | Project scope | Явный вызов |
| --- | --- | --- | --- |
| Codex | `~/.agents/skills/execution-state/` | `.agents/skills/execution-state/` | `$execution-state` |
| Claude Code | `~/.claude/skills/execution-state/` | `.claude/skills/execution-state/` | `/execution-state` |
| Gemini CLI | `gemini skills link <PATH>` | `.gemini/skills/execution-state/` или `.agents/skills/execution-state/` | явно попросить активировать `execution-state` |
| Другой агент | По документации агента | По документации агента | native invocation либо абсолютный путь к `SKILL.md` |

Актуальные каталоги и команды описаны в официальных руководствах
[Codex](https://developers.openai.com/codex/skills),
[Claude Code](https://code.claude.com/docs/en/slash-commands) и
[Gemini CLI](https://geminicli.com/docs/cli/using-agent-skills/).

Проверь установку и runtime:

```bash
python3 <PATH>/execution-state/scripts/statectl.py \
  route --source standalone --expected-turns 3 --adapter manual

python3 <PATH>/execution-state/scripts/statectl.py \
  runtime-probe --adapter auto
```

Первый вызов должен вернуть `decision: "passthrough"`. Второй не делает model
request: он запускает только короткий `<cli> --version`. Если найдено несколько
CLI, `auto` намеренно вернёт `manual`; передай нужный adapter явно. Probe пока
не проверяет минимальную версию и каждый launch flag, поэтому перед первым
reset дополнительно проверь `<cli> --help` и сформированный `runtime-plan`.

## Использование

- [Инструкции skill](SKILL.md)

Единственный публичный helper — `scripts/statectl.py`. Он выбирает route,
обслуживает schema/revision, создаёт packet и строит launch plan. Он не запускает
платный agent CLI самостоятельно.

Отдельные prompt templates временно не поставляются. Вызови skill нативным
синтаксисом агента и опиши задачу обычным сообщением. Текущая директория,
`base-agent`, auto adapter, проверяемый результат и минимальные критерии
coordinator определяет самостоятельно.

`packet` — изменяющая операция: она повышает revision, создаёт единственный
активный `worker_lease` и возвращает `run_id`. `runtime-plan` принимает packet
только вместе с соответствующим `--id` либо `--state`. Результат worker
принимается через `observe`, `complete` или `block` с новой revision и тем же
`--run-id`; повторный packet до снятия lease отклоняется.

Для сложной standalone-задачи task JSON может включать:

```json
{
  "id": "auth-idempotency",
  "title": "Исправить replay регистрации",
  "done_when": ["Повтор возвращает исходный ответ"],
  "kind": "implementation",
  "cohesion_key": "auth-http",
  "affected_areas": ["auth", "http"],
  "reads": ["internal/app/app.go"],
  "writes": ["internal/app/protection.go"],
  "contracts": ["одинаковый key с другим body возвращает conflict"],
  "regression_checks": ["idempotency-replay", "idempotency-conflict"],
  "requires_bridge": true
}
```

`complete` примет такую задачу только с passed structured check для каждого ID.
После cross-area изменения следующий task должен иметь `kind=integration`.
Каждые восемь chunks по умолчанию требуется `architecture-review`. Команда
`context-map-update` сохраняет только path/purpose/areas/symbols, а worker packet
выбирает из карты максимум 16 релевантных записей.

`architecture-review` принимает JSON с отдельными `verdict`, `blockers`,
`planned_gaps`, `recommendations` и `checks`, но сохраняет в state компактную
типизированную сводку не более 1 KiB. Полный ответ остаётся в raw-артефакте.
При `blocked` standalone ledger получает recovery chunk перед обычной задачей;
после его выполнения обязательный re-review не позволяет незаметно снять gate.

Изменяемые операционные записи хранятся на кратком техническом английском:
наблюдения, summaries/evidence, blockers, причины checkpoint, review и описания
context map. Дословные требования пользователя или OpenSpec остаются на языке
источника, чтобы перевод не менял смысл acceptance criteria и контрактов.
Helper ограничивает UTF-8-размер, но не пытается автоматически распознавать
язык и не запрещает Unicode. Полная граница полей описана в
[compact state schema](references/state-schema.md).

State другой schema version текущий controller не изменяет. Незавершённую
старую работу заверши совместимым release либо начни новый state; автоматической
миграции и незаметного переписывания данных нет.

Для стороннего CLI передай `--adapter <ID> --manifest <PATH>` и только после
проверки manifest добавь `--trust-custom-adapter`. Этот флаг разрешает безопасный
version probe указанного executable, но не разрешает model run, bypass
permissions или другие внешние действия.

Подробности:

- [compact state schema](references/state-schema.md);
- [универсальный runtime contract](references/runtime-contract.md);
- [CLI adapters](references/cli-adapters.md);
- [OpenSpec-lite](references/openspec-integration.md);
- [worker protocol](references/worker-protocol.md).

## Проверка качества

Актуальный standalone benchmark сравнивает только `execution-state 1.0.0` и
обычный AI. Каждый вариант стартует в пустом проекте и новой session;
OpenSpec/SDD не входит в измеряемые переменные. Сценарий, метрики и независимый
black-box verifier описаны в
[quality benchmark](../../benchmarks/go-auth-service/quality/README.md).

Старые прогоны и межверсионные сравнения не входят в репозиторий. Новые
результаты считаются отдельными наблюдениями и не используются как статистика
релиза, пока пользователь явно не решит сохранить конкретный отчёт.
