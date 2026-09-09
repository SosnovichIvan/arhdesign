## Why

Долгие OpenSpec Apply-задачи должны сохранять компактное проверяемое состояние между semantic chunks и сменами контекста, не превращая чат или `.ai/context/` во второй task ledger.

## From → To

- **From:** OpenSpec задаёт change и tasks, но не использует единый runtime state protocol.
- **To:** полный skill `execution-state` доступен локально в `.ai/skills/`; каждый OpenSpec Apply сначала проходит его routing.

## Scope

- Перенести полный каталог skill из `/Users/ivansosnovich/Documents/codex/skils/skills/execution-state` в `.ai/skills/execution-state/`.
- Закрепить обязательный `$execution-state` routing до Apply каждой принятой OpenSpec-задачи.
- Описать режимы `passthrough`, `lite` и `reset`, границу источников истины и правила отметки checkbox.
- Игнорировать локальный runtime state `.execution-state/` в Git.

## Non-goals

- Не менять формат OpenSpec change, не создавать task JSON и не запускать отдельного worker автоматически.
- Не применять `execution-state` к Explore, Propose, Review или не принятым change.

## Version

- Previous: отсутствует.
- Target: v1.0.0.
- Level: MINOR — новое обязательное правило исполнения без изменения продукта.
