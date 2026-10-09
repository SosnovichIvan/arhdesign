# Универсальный runtime contract

Этот контракт не зависит от модели, API или agent CLI. Уже запущенный skill не
может универсально удалить собственную историю: реальный reset выполняет
внешний runtime либо новая сессия через adapter.

## Слои

| Слой | Ответственность |
| --- | --- |
| Skill | Декомпозиция, contracts, границы chunk и checkpoint policy |
| `statectl` | State machine, quality gates, context map, revision/lease и packet |
| CLI-adapter | Только discovery, capabilities и безопасный launch plan |
| Worker | Реализация одного chunk и структурированный результат |

Adapter не трактует цель, не принимает результат и не изменяет state или
`tasks.md`. Vendor-specific флаги не попадают в state schema.

## Capability handshake

Runtime сообщает только подтверждённые возможности; неизвестное означает
`false`: `fresh_context`, `in_place_compaction`, `machine_output`,
`session_persistence_control` и `usage_metrics`. Prompt transport и argv
описываются launch-секцией adapter manifest, а не state machine.

Порядок выбора adapter: явно заданный ID, переменная wrapper, единственный
доступный встроенный adapter, доверенный declarative manifest, затем `manual`.
При неоднозначности не выбирай CLI молча.

## Стратегии

1. `fresh-process` — переносимый основной вариант: новый неперсистентный CLI-
   процесс получает один worker request.
2. `native-compact` — опционально, только для документированной управляемой
   compaction; это не строгий state-only reset.
3. `manual-handoff` — packet готов, пользователь или внешний runner открывает
   новый контекст.
4. `embedded-lite` — reset недоступен; checkpoint используется для
   возобновления, но экономия внутри текущей сессии не заявляется.

Наличие автоматической compaction при переполнении не равно способности
выполнить reset в выбранной semantic boundary.

## Lifecycle одного chunk

```text
probe runtime → choose strategy → begin chunk → implement and verify contracts
→ update context map/state → architecture/bridge gate → checkpoint
→ packet/lease → bound runtime-plan
→ compact/fresh/manual → accept result/release lease
```

Не запускай новый контекст для каждого чтения или shell-действия. Chunk должен
давать законченный проверяемый результат и обычно включает несколько внутренних
agent turns. Закончи его раньше при блокере, запросе permission, смене
подсистемы или превышении установленного бюджета.

Reset выгоден, когда ожидаемая повторная передача одноразовой истории больше
cold start и rehydration packet. Для соседних chunks с одним `cohesion_key`
используй `continue`; при смене ключа — `reset`, если capability подтверждена;
иначе `checkpoint`. Это решение хранится как vendor-neutral рекомендация, а
конкретный adapter выбирает доступный механизм.

## Передача

До смены контекста controller обязан:

1. принять только проверяемые факты и structured checks для всех объявленных
   regression contracts;
2. сохранить валидный checkpoint с одним `next_action`;
3. записать worker packet через `statectl packet`, передав
   `--expected-revision`, `--run-id` и `--output`; операция повышает revision и
   создаёт единственный активный `worker_lease`;
4. построить `runtime-plan` с `--id` либо `--state`, чтобы controller проверил
   packet против текущего state и lease;
5. передать adapter только путь проекта, prompt/packet и безопасные runtime
   параметры;
6. не передавать предыдущие сообщения, рассуждения и полные логи.

Checkpoint считается reset-ready только когда state валиден и укладывается в
лимит, `next_action` задаёт одно действие, изменённые artifacts перечислены,
объявленные regression checks имеют structured status `passed`, а observation
содержит факт вместо сырого лога. При blocker запиши недостающий ввод и один
способ продолжения. Не создавай checkpoint посреди миграции, интерактивной
операции или непроверенного изменения.

Выбирай `continue` для соседнего chunk с тем же `cohesion_key` и ещё полезным
локальным context; `reset` — при смене ключа и подтверждённой reset capability;
`checkpoint` — для manual handoff или lite. Resume/fork не считаются reset,
если переносят предыдущую историю.

Перед packet controller запрещает пропуск обязательного architecture review и
запрещает обычный implementation chunk, пока cross-area изменение ожидает
`kind=integration`. Релевантные записи context map выбираются по areas и
рабочим путям; весь индекс и содержимое файлов в packet не копируются.

Worker возвращает result envelope с `run_id` и `based_on_revision`. Controller
требует, чтобы они совпали с lease и post-packet revision, проверяет изменённые
артефакты и `done_when`, после чего вызывает `observe`, `complete` или `block` с
этими `--run-id` и `--expected-revision`. Операция снимает lease и единолично
обновляет state; только `complete` обновляет OpenSpec checkbox. Невалидный
envelope не завершает задачу; допускается максимум одна попытка восстановить
формат.

После ответа worker lease снимается только через revision-bound `observe`,
`complete` или `block`. Не бросай state с активным lease и не создавай
заменяющий packet.

## Безопасность

- Adapter запускает процесс только argv-массивом, без shell interpolation.
- Prompt передаётся через stdin или файл, а не как собранная shell-строка.
- Adapter наследует действующие project rules и permissions; обход sandbox и
  approvals не включается автоматически.
- Custom manifest не содержит parser hooks, секреты или исполняемый код.
- Custom executable не получает даже `--version` без явного
  `--trust-custom-adapter`; этот opt-in не разрешает model run.
- Версия CLI проверяется без model request; реальные smoke-runs выполняются
  только явно, поскольку расходуют время и токены.

Детали discovery и расширения описаны в
[cli-adapters.md](cli-adapters.md), формат worker/result — в
[worker-protocol.md](worker-protocol.md).
