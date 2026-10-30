# OpenSpec-lite

OpenSpec задаёт намерение и обязательства, execution-state хранит только
активное выполнение. CLI-adapter не влияет на это разделение.

## Источники истины

| Данные | Источник |
| --- | --- |
| Scope и причина изменения | `proposal.md` |
| Нормативные требования | `specs/` |
| Архитектурные решения | `design.md` |
| Task graph и статусы | `tasks.md` |
| Активный chunk, observation, последнее evidence, checkpoint и worker lease | `state.json` |
| Подробная история реализации | код, проверки и git; не worker packet |

Не копируй OpenSpec-текст в state и не создавай `tasks/*.json`. Active task
содержит только ID, короткое название, source refs, рабочие файлы и локальные
`done_when`.

## Предусловия

Используй state только на Apply после принятого change. Для Explore, Propose и
Review сохраняй знания в соответствующие OpenSpec-артефакты. Если change
неполон или противоречив, не создавай state.

Исполняемые строки `tasks.md` должны иметь checkbox и стабильный ID:

```markdown
- [ ] 1.1 Добавить модель
- [ ] 1.2 Реализовать middleware
- [ ] 1.3 Выполнить проверку
```

## Выполнение

1. `statectl init --source openspec` выбирает первую незавершённую задачу и
   создаёт один compact overlay.
2. `statectl packet` повышает revision и создаёт единственный lease; worker
   читает только source refs текущего chunk.
3. Несколько соседних задач можно объединить в chunk лишь при общих файлах и
   одной проверке, без конфликтующих зависимостей.
4. Если chunk передан через packet, `statectl complete` требует evidence,
   совпадающие `--run-id` и post-packet `--expected-revision`, отмечает текущий
   checkbox и выбирает следующую незавершённую задачу.
5. Если реализация обнаружила новое требование или изменение дизайна,
   заблокируй chunk и сначала обнови нормативный OpenSpec-артефакт.
6. Verify выполняется по правилам проекта; Archive не следует из completion и
   требует обычного разрешения.

Worker никогда не редактирует checkbox самостоятельно. Controller проверяет
lease, revision и evidence, затем выполняет детерминированный переход и снимает
lease. При сбое между файлами последующая `statectl validate` должна выявить
рассинхронизацию до продолжения.

## CLI portability

OpenSpec-режим не выбирает Codex, Claude или иной runtime. После checkpoint
тот же packet может быть передан другому adapter, если новый worker видит тот
же project root и OpenSpec change. Adapter заново проходит capability
negotiation и не сохраняется как нормативная часть state.

Для добавления этих правил в проект перенеси разделы «Источники истины»,
«Предусловия» и «Выполнение» в агентную документацию проекта, адаптировав только
пути и OpenSpec change ID. Не копируй runtime state в нормативные артефакты.
