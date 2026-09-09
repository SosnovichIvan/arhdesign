---
name: arhdesign-review
description: Ревьюировать изменения arhDesign на соответствие OpenSpec, FSD, доступности, адаптивности и темам. Использовать для code review и проверки готовности UI-задач.
---

# Review arhDesign

## Источники проверки

Сопоставить изменение с `projectrules.md`, релевантными OpenSpec spec/design/tasks и Figma, если изменение визуальное. Не считать предположение требованием.

- Для frontend-ревью прочитать [frontend-review.md](references/frontend-review.md).
- Для Go API прочитать `.ai/rules/backend/README.md`, `.ai/rules/security/README.md`, `.ai/rules/contracts/README.md` и `.ai/rules/testing/README.md`.
- Если задача затрагивает UI или содержит Figma-ссылку/макет, обязательно прочитать [design-review.md](references/design-review.md) и провести визуальную сверку.

## Базовый чек-лист

- Поведение реализовано в границах принятого OpenSpec; v1 не получает CMS, auth, аналитику или формы без отдельного согласования.
- FSD: зависимости направлены только вниз по слоям; нет импортов между слайсами одного слоя; общий код не лежит в `shared`, если он доменный или одноразовый.
- UI: production-разметка использует Tailwind; нет CSS Modules, styled-components, inline-style обходов и локальных дублей shared primitives.
- Naming: папки, файлы, props, hooks и локальные identifiers компонентов используют `camelCase`; public React component identifiers используют `PascalCase` только как требование JSX.
- Reuse: примитивы повторного использования живут в `src/shared/ui/`, имеют public API и импортируются по нему.
- Компоненты продукта с двумя и более применениями вынесены в `src/shared/components/`, имеют `index.ts`; локальные копии являются blocking review finding.
- UI: нет переполнения в Desktop/Tablet/Mobile; light/dark меняют токены, а не структуру; клавиатурная навигация, фокус и контраст соответствуют WCAG 2.2 AA.
- Для изменённого сценария выполнены подходящие проверки: typecheck, тесты, axe/Playwright и, для UI, визуальная сверка с Figma.
- Coverage: authored frontend имеет не менее 90% lines/functions/branches/statements, Go backend — не менее 90% atomic statement coverage и покрытые table-driven decision branches; generated/config/type/migration/fixture paths исключены обоснованно, а снижение покрытия — blocking finding.
- Backend: контракт обновлён до кода, handler не обращается к БД, secrets и PII не попадают в код/логи, пройдены `go vet` и `go test -race`.

## Формат результата

Сначала перечислить только воспроизводимые замечания с файлом, риском и кратким исправлением. Если замечаний нет — явно назвать выполненные проверки и оставшиеся непроверенные зоны.
