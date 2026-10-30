# Next.js

Читайте вместе с `react.md`, когда подтверждён Next.js.

## Router и execution boundary

- Сначала определи App Router или Pages Router; не смешивай их соглашения.
- Зарезервированные route-файлы сохраняют точные имена framework: `page`,
  `layout`, `loading`, `error`, `not-found`, `route` и их допустимые extensions.
  Route groups `(group)`, private folders `_folder` и dynamic segments `[id]`
  не переименовывай по общему naming fallback.
- Обычные components/hooks вне route conventions следуют `react.md` и общему
  code-conventions reference.
- Сохраняй server/client boundary. Добавляй `use client` только когда компоненту
  реально нужны browser APIs, state, effects или client-only hooks.
- В App Router `use client` задаёт entrypoint клиентского module graph: не
  размечай им layout/page целиком, если интерактивна только вложенная часть.
- Props через server-to-client boundary должны быть сериализуемыми.
- Не передавай secrets и server-only modules через client graph; используй
  принятые server-only/client-only guards проекта.

## Данные, mutations и route states

- Используй принятый в проекте способ data fetching, caching, revalidation и
  mutations; не переносись между моделями ради локального изменения.
- Не предполагавай cache semantics по памяти: проверь Next.js version и локальную
  конфигурацию. Revalidation должна соответствовать freshness требованиям.
- Параллелизуй независимые server reads и ставь meaningful Suspense/loading
  boundary вокруг действительно медленной части, избегая request waterfall.
- Server Action/route handler повторно валидирует input, authentication и
  resource authorization; скрытое поле формы или закрытая кнопка не защищают.
- После mutation обнови ровно необходимые cache tags/paths или client query.
- Для route segment реализуй существующие loading/error/not-found boundaries.
- Сохраняй middleware, auth и runtime choice (Node/Edge) целевого route.

## Проверка

- Проверяй hydration, direct navigation, refresh и production build.
- Не меняй rendering mode страницы без оценки cache, SEO и latency последствий.
- Проверь server/client import graph, metadata затронутого public route и работу
  deployment target: Node, Edge, static export или adapter имеют разные границы.
