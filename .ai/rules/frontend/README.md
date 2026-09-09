# Frontend UI rules

## Tailwind-only UI

- Весь production UI реализуется Tailwind utility classes.
- Цвета, типографика, spacing, radii, shadows и breakpoints берутся из Tailwind theme и semantic CSS variables Light/Dark.
- CSS Modules, styled-components, inline style-объекты и локальные CSS-файлы компонентов запрещены.
- Разрешены только глобальный Tailwind entrypoint, semantic CSS variables для тем и обязательные стили внешней библиотеки.

## Reusable primitives

- Папки, файлы, props, hooks и локальные identifiers компонентов именуются в `camelCase`; публичные React component identifiers используют `PascalCase`, так как это требование JSX. Не использовать kebab-case или snake_case для имён компонентов.
- Базовые UI-примитивы живут только в `src/shared/ui/`: Button, IconButton, Input, Textarea, Dialog, CarouselControl, Container, Typography и аналогичные.
- Любой компонент продукта, используемый более одного раза, выносится в `src/shared/components/<component-name>/` и экспортируется через `index.ts`. Это относится к повторяемым составным блокам, которые не являются базовыми primitives.
- Каждый primitive имеет собственную папку, `ui/`, при необходимости `model/`/`lib/`, и публичный export через `index.ts`.
- Страницы, widgets и features импортируют примитивы только из `@/shared/ui/<primitive>` или общего barrel API; локальные дубли Button/Input/Dialog запрещены.
- Перед сдачей выполнить reuse audit: найти повторяющуюся разметку/компоненты и подтвердить, что при двух и более применениях они импортируются из `@/shared/components/<component-name>`.
- Составные domain-specific блоки не переносить в `shared`: они остаются в feature/entity/widget и используют primitives.
- Новый primitive добавляется только после проверки минимум двух реальных точек повторного использования; одноразовый UI остаётся рядом с потребителем.
