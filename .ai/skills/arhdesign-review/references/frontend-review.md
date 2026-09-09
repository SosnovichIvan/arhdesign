# Frontend review

Использовать при ревью TypeScript/React/Next.js-кода. Проверять только то, что относится к изменению.

## Нейминг

- Папки, файлы, FSD-слайсы, переменные, props, обработчики и функции именовать в `camelCase`: `projectCard`, `themeToggle`, `handleSubmit`.
- Реализующие React-компоненты экспортировать в `PascalCase`: `ProjectCard`. JSX распознаёт компоненты только по имени с прописной буквы; `camelCase` допустим для пути/файла, но не для JSX-идентификатора компонента.
- Custom hooks именовать `use` + `PascalCase`: `useThemePreference`. Типы, интерфейсы и enum — `PascalCase`; константы, не являющиеся перечислениями, — `camelCase`.
- Next.js file conventions сохраняют их зарезервированные имена: `page.tsx`, `layout.tsx`, `loading.tsx`, `error.tsx`, `route.ts`.

## Архитектура и типы

- Проверить правило FSD: зависимость разрешена только к слоям ниже; слайсы одного слоя изолированы. Наружу слайс отдаёт public API. Исключение `@x` допустимо только для явно документированной связи entity-to-entity.
- Не создавать `entities` или `features` ради одной страницы; оставить одноразовый код рядом с её slice.
- В `strict` TypeScript не допускать неявный `any`; явный `any` — только с коротким обоснованием, когда `unknown` или точный тип неприменимы. Promise должен быть `await`-нут, возвращён, обработан или намеренно `void`-нут.

## Рендеринг и Next.js

- Server Component остаётся вариантом по умолчанию. `'use client'` оправдан только state, event handlers, effects, browser APIs или зависимостями, которые требуют клиента; граница должна быть максимально глубокой.
- Не передавать из Server Component в Client Component несерилизуемые props. Server-only и client-only код не смешивать; секреты и серверные SDK не попадают в клиентский bundle.
- В списках использовать стабильный ключ из данных, не индекс и не случайное значение, когда порядок или состав могут меняться.
- Рендерить семантический элемент по назначению: `button` для действия, `a`/`Link` для навигации, нативные поля для ввода. Интерактивный элемент имеет доступное имя, видимый focus и работает с клавиатурой.
- Для изображения проверять осмысленный `alt` либо `alt=""` для декоративного, заданное соотношение сторон/размеры и `sizes` для адаптивного `fill`. LCP-медиа не должно лениво появляться после первого экрана.
- Проверить состояния загрузки, пустого результата и ошибки, если их допускает сценарий; не допускать hydration mismatch, layout shift и client-only API в Server Component.

## Проверки

- Запустить доступные lint, typecheck, unit/component tests и e2e для затронутого сценария.
- Для маршрутов проверить server-rendered HTML, metadata и отсутствие ошибок в console/network, если это входит в изменение.
- Для UI дополнительно применить `design-review.md`.

## Первичные источники

- [FSD: layers and import rule](https://fsd.how/docs/get-started/overview/)
- [FSD: public API and `@x`](https://fsd.how/docs/reference/public-api/)
- [Next.js: Server and Client Components](https://nextjs.org/docs/app/getting-started/server-and-client-components)
- [React: component and hook naming](https://react.dev/learn/reusing-logic-with-custom-hooks)
- [React: stable list keys](https://react.dev/learn/rendering-lists)
- [TypeScript strict options](https://www.typescriptlang.org/docs/handbook/compiler-options.html)
- [typescript-eslint: no explicit any](https://typescript-eslint.io/rules/no-explicit-any/)
- [typescript-eslint: no floating promises](https://typescript-eslint.io/rules/no-floating-promises/)
- [Next.js Image](https://nextjs.org/docs/app/api-reference/components/image)
