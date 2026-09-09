---
name: execution-state
description: "Координирует долгие агентные задачи через компактное состояние, контрактные semantic chunks, context map, quality gates и смену контекста; работает с OpenSpec или самостоятельно в любом agent CLI. Применять только при явном вызове в синтаксисе текущего CLI."
---

# Выполнение через состояние

Используй execution state как переносимую рабочую память между смысловыми
частями долгой задачи. Core не зависит от Codex, Claude или другого агента:
текущий runtime выбирается по возможностям через CLI-adapter.

## Входные условия

- Skill должен быть явно вызван в синтаксисе текущего агента.
- Пользователь обязан задать только задачу. Выведи из неё минимальный
  проверяемый результат и критерии завершения; запроси уточнение лишь когда
  неоднозначность существенно меняет результат или разрешённые действия.
- Если отдельные границы не указаны, границами остаются исходный запрос,
  текущие permissions и правила среды. Не проси пользователя заполнять поля
  внутренней schema.
- Ссылка на skill или профиль реализации передаётся как непрозрачное значение;
  это может быть `$name`, `/name`, путь к правилам или `base-agent`.
- Для standalone-запуска по умолчанию используй текущую директорию,
  `base-agent` и auto-выбор adapter с безопасным fallback в manual handoff.
- Сам skill не расширяет полномочия и не включает обход permissions.

Во время обычного выполнения не читай исходники helpers: запускающий запрос и
этот `SKILL.md` содержат нужные правила. Исходники helper читай только для
диагностики ошибки.

## Маршрутизация

Сначала оцени число semantic chunks, ожидаемый handoff и объём одноразовой
истории. Источник и профиль выполнения — независимые оси:

- `source`: `standalone` или `openspec`;
- `execution.profile`: `lite` или `reset`;
- `passthrough` не создаёт state вообще.

Для очевидно короткой связной задачи без handoff выбери `passthrough` и сразу
используй skill реализации. На этом workflow execution-state заканчивается:
не читай его references, не запускай helpers и не создавай state. В OpenSpec
продолжай обычный Apply по его источникам истины. В сомнительном случае
используй `statectl route`. Не обещай экономию токенов в `lite`: этот профиль
даёт только checkpoint.

Только если для OpenSpec Apply после маршрутизации выбран `lite` или `reset`,
прочитай [правила OpenSpec-lite](references/openspec-integration.md). Для
`reset` дополнительно прочитай
[контракт runtime](references/runtime-contract.md). Остальные references
открывай только когда соответствующая операция действительно нужна.

## Единственный helper

Используй только `scripts/statectl.py`, найденный относительно этого `SKILL.md`.
Запускай его доступным Python 3 interpreter; не предполагай путь проекта или имя
конкретного agent CLI. Не вызывай все подкоманды с `--help`.

Основные операции:

```text
statectl route
statectl init
statectl begin
statectl observe
statectl complete | statectl block
statectl checkpoint
statectl architecture-review
statectl context-map-update
statectl packet --expected-revision <N> --run-id <ID> --output <file>
statectl validate
statectl version
statectl runtime-probe | statectl runtime-plan
```

Точный интерфейс и компактная schema описаны в
[state-schema.md](references/state-schema.md). Helper сам проверяет revision,
переходы и лимиты; не создавай ручные JSON Patch-файлы и не запускай отдельную
валидацию после каждой успешной операции.

State другой schema version не изменяй и не мигрируй автоматически: заверши его
совместимой версией skill либо начни новый state. Не редактируй state вручную.

## Рабочий цикл

1. Создай или возобнови state только после выбора `lite` либо `reset`.
2. До выполнения разложи работу на semantic chunks по
   [контракту декомпозиции](references/microtask-decomposition.md). Для каждого
   сложного chunk укажи `affected_areas`, `reads`, `writes`, `contracts`,
   `regression_checks`, `cohesion_key` и при необходимости `requires_bridge`.
3. Выполняй один semantic chunk: законченный результат вместе с его узкой
   проверкой, обычно несколько внутренних tool/model шагов. Соблюдай глобальные
   `quality.invariants` и не ограничивай проверку только локальным `done_when`.
4. Обнови state один раз после значимого результата, блокера или смены
   подсистемы. Новые операционные записи формулируй кратко на английском по
   правилам ниже. Не записывай чтения файлов и внутренние рассуждения.
5. После изменения устойчивой структуры обнови компактную `context-map.json`.
   Не копируй в неё код и логи: только путь, назначение, areas и symbols.
6. Завершай chunk только после всех объявленных `regression_checks`, переданных
   как structured checks. Свободный evidence допустим лишь когда checks не были
   объявлены. OpenSpec checkbox меняет deterministic core, а не worker.
7. Если controller требует architecture review, проверь границы, wiring,
   публичные контракты и дублирование. Передай structured review с `verdict`,
   `blockers`, `planned_gaps`, `recommendations` и `checks` через
   `architecture-review --review-json`. `passed` снимает gate; `blocked`
   создаёт standalone recovery chunk или блокирует OpenSpec state.
   Cross-area chunk должен переходить в явно запланированный `integration`
   chunk; controller блокирует обычную реализацию при pending bridge.
8. Перед сменой контекста создай валидный checkpoint. Затем один раз вызови
   `packet`: эта операция повышает revision и создаёт `worker_lease`, связанный
   с `run_id`. Сохрани `run_id` и новую revision из ответа helper.
9. Используй `quality.next_handoff`: `continue` сохраняет текущий context для
   связного следующего chunk, `reset` выбирается при смене cohesion key и
   подтверждённой capability, `checkpoint` означает manual/lite переход.
10. Построй `runtime-plan`, обязательно привязав его к тому же state через
   `--id` либо `--state`; один packet без state недостаточен.
11. Выбери стратегию только по подтверждённым capabilities runtime:
   fresh process, управляемая compaction, manual handoff или `lite` fallback.
12. Новый worker получает packet, релевантные source refs, contracts и выбранные
   context-map entries, но не
   историю разговора, предыдущие рассуждения или сырые логи.
13. Принимая ответ worker, вызови `observe`, `complete` или `block` с
   `--expected-revision`, равной `based_on_revision` packet, и с тем же
   `--run-id`. Это снимает lease; без совпадения state не меняется.

Fresh process запускай через adapter как argv-массив без shell и только в рамках
исходных permissions. Если adapter неизвестен или выбор неоднозначен, используй
manual handoff; не угадывай флаги CLI. Подробнее:
[cli-adapters.md](references/cli-adapters.md).

Custom manifest не доверяй неявно. Флаг `--trust-custom-adapter` допустим только
после явного выбора и проверки manifest: он разрешает запуск указанного
executable с `--version`, хотя сам `runtime-plan` model request не выполняет.

## Инварианты компактности

- `state.json` — не более 8 KiB, worker packet — не более 12 KiB.
- Последнее наблюдение — не более 2 KiB; полный вывод остаётся в отдельном логе.
- OpenSpec `tasks.md` остаётся единственным task ledger; task JSON не создаются.
- В worker packet нет завершённых задач, истории и runtime/vendor metadata.
- Один coordinator изменяет state; worker возвращает revision-bound result по
  [worker protocol](references/worker-protocol.md).
- Объявленные global invariants действуют на каждый chunk; объявленные
  regression checks нельзя заменить описанием «проверено».
- Context map — навигационный индекс, а не второй источник требований.
- Изменяемый операционный текст хранится на кратком техническом английском:
  observations, summaries/evidence, blockers, checkpoint reasons, review и
  context-map descriptions. Дословные требования пользователя и источников
  сохраняются на исходном языке; не переводи `goal`, task title, `done_when`,
  constraints, contracts или invariants, если они импортированы либо их точная
  формулировка важна. Это протокольное правило, а не ASCII-ограничение.
- `quality.last_review` — компактная запись не более 1 KiB; полный текст review
  остаётся во внешнем артефакте и не переносится в worker packet.
- Для одного state одновременно существует не более одного `worker_lease`;
  повторный packet до приёмки результата запрещён.
- Секреты, access tokens, пароли и персональные данные в state не записываются.
