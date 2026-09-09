---
name: arhdesign-frontend
description: Реализовывать фронтенд arhDesign на Next.js по SDD и актуальным правилам Feature-Sliced Design. Использовать для создания или изменения production UI.
---

# Frontend arhDesign

## Перед реализацией

- Прочитать `projectrules.md`, релевантный change в `openspec/changes/` и целевой Figma-фрейм.
- Не начинать production-реализацию, пока OpenSpec-change и дизайн-концепция не приняты; если это неясно, сообщить блокер.
- Зафиксировать проверяемый результат в OpenSpec-задаче или предложить отдельный change, если меняется наблюдаемое поведение.

## Минимальная структура

Использовать `src/` и слои FSD: `app`, `pages`, `shared`. Добавлять `entities` и `features` только при появлении самостоятельной доменной сущности или повторно используемого пользовательского действия. Не создавать `widgets` по умолчанию; применять их только для крупного самостоятельного блока, который используется более чем на одной странице.

```text
src/
  app/                 # Next.js route composition, providers, global styles
  pages/<page>/        # page slice: ui, model, lib, api при необходимости
  entities/<entity>/   # доменная сущность, когда она действительно выделилась
  features/<feature>/  # самостоятельное пользовательское действие
  shared/              # ui, lib, api, config, assets
```

- Слои могут импортировать только слои строго ниже: `app → pages → widgets → features → entities → shared`.
- Слайсы одного слоя не импортируют друг друга; наружу экспортировать только через public API (`index.ts`).
- `app` и `shared` не имеют слайсов и могут быть разделены на технические сегменты.
- Не вводить слой `processes`: он устарел в актуальной FSD.
- Не абстрагировать одноразовый код: держать его рядом с единственной потребляющей страницей.

## UI implementation

- Папки, файлы, props, hooks и локальные identifiers компонентов именуются в `camelCase`; публичные React component identifiers — `PascalCase`, требуемый JSX.
- Весь production UI реализуется только Tailwind utility classes. Использовать semantic CSS variables и Tailwind theme для Light/Dark, typography, spacing, radius и breakpoints.
- Запрещены CSS Modules, styled-components, inline style-объекты и локальные CSS-файлы компонентов; разрешён только глобальный Tailwind entrypoint и theme variables.
- Примитивы, используемые более чем в одном месте, создавать в `src/shared/ui/` и экспортировать через `index.ts`. Button, IconButton, Input, Textarea, Dialog, CarouselControl, Container и Typography не дублируются внутри pages/widgets/features.
- Составной компонент продукта, используемый два и более раза, выносить в `src/shared/components/<component-name>/` с public API `index.ts`; не оставлять его копии рядом с потребителями.
- Составные или доменные блоки остаются в своём FSD слое и импортируют shared primitives через public API.

## Локальные требования проекта

- Next.js App Router, TypeScript, React Server Components по умолчанию; клиентские компоненты — только для необходимой интерактивности.
- Данные интерфейса получают через доменные типы и `ContentRepository`; инфраструктурные SDK не импортируются в UI.
- Для API использовать типы, сгенерированные из `openapi/`; ручные DTO для transport-слоя запрещены.
- Пользовательская форма отправляет данные только в собственный Go API; ключи email/Telegram и антиспам-логика не попадают во frontend.
- Светлая и тёмная темы используют одни семантические CSS-токены; не дублировать разметку ради темы.
- Каждую UI-правку сверять в Desktop, Tablet, Mobile, Light и Dark, если состояние применимо.
- До сдачи выполнить релевантные typecheck, tests, Vitest V8 coverage >=90% по lines/functions/branches/statements и визуальную проверку; не заявлять о прохождении непроведённых проверок.
- В review проверить, что не добавлены локальные стили или дубли shared primitives.
- Выполнить reuse audit: любой повторяемый компонент продукта импортируется из `shared/components`.
