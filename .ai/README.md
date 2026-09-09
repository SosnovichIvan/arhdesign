# AI workspace

Единая точка входа для работы ИИ в `arhDesign`.

## Состав

- `skills/arhdesign-frontend/` — минимальные правила реализации Next.js по SDD и Feature-Sliced Design.
- `skills/arhdesign-backend/` — Go REST API, OpenAPI, PostgreSQL и VDS/Docker Compose.
- `skills/arhdesign-review/` — проверка изменений до сдачи.
- `skills/arhdesign-designer/` — работа с согласованным Figma-дизайном и дизайн-ревью.
- `skills/execution-state/` — обязательный routing и компактное runtime state для Apply принятых OpenSpec change.
- `context/` — актуальное состояние продукта, ключевые решения и журнал пользовательской обратной связи.
- `rules/` — backend, security, contracts, testing, devops и documentation для production-реализации.
- `rules/frontend/` — Tailwind-only UI и правила reusable primitives.

## Порядок применения

1. Сначала прочитать [`../projectrules.md`](../projectrules.md) и релевантные OpenSpec-артефакты.
2. Прочитать [`context/README.md`](context/README.md) и файл контекста, относящийся к задаче.
3. Перед Apply принятого OpenSpec change явно загрузить `$execution-state` и выполнить routing с `source=openspec`. Для короткого связного Apply route может выбрать `passthrough`; иначе использовать `lite` или `reset` state.
4. Для работы в Figma загрузить `arhdesign-designer`; для интерфейса — `arhdesign-frontend`; для Go API — `arhdesign-backend`.
5. Для ревью, перед сдачей или при запросе проверки загрузить `arhdesign-review`.

`tasks.md` остаётся единственным task ledger. Runtime state в `.execution-state/` хранит только активный semantic chunk, evidence и checkpoint, не дублирует OpenSpec и не попадает в Git.

`projectrules.md`, OpenSpec и явные запросы пользователя имеют приоритет над локальными skills. Эта директория не является production-кодом и не заменяет правила Codex.
