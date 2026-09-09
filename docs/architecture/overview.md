# Техническая архитектура

## Рекомендация

Next.js App Router + TypeScript как модульный монолит. Публичный контент в v1 статически генерируется из валидируемых файлов. Это обеспечивает быстрый server-rendered HTML и не мешает позднее добавить CMS, `/admin` и `/account`.

## Модули

```text
app (routes, metadata, composition)
  ├── portfolio (projects, services, profile)
  ├── search (metadata, sitemap, JSON-LD)
  ├── theme (tokens, preference, no-flash bootstrap)
  └── contact (channels; form only when approved)

domain (framework-independent schemas and types)
  └── Project, Service, Profile, Contact, SeoFields

infrastructure
  ├── content/file-adapter        v1
  ├── content/cms-adapter         future
  ├── media                       transforms/storage boundary
  └── analytics                   optional adapter
```

UI получает данные только через доменные типы. CMS SDK, session API и база данных не импортируются в презентационные компоненты.

## Контентная модель

`Project`: id, slug, status, title, excerpt, location, year, typology, services, role, area, cover, gallery, challenge, solution, credits, seo, publishedAt, updatedAt.

`Media`: src, width, height, aspectRatio, alt, caption, credit, focalPoint, kind.

`Service`: slug, title, summary, deliverables, process, seo.

`Profile`: name, role, baseLocation, serviceAreas, bio, portrait, credentials, publications, socialLinks.

## Этапы развития

| Этап | Хранилище | Публикация | Доступ |
|---|---|---|---|
| v1 | Git-backed content + object/static media | build/deploy | public |
| CMS | PostgreSQL + object storage | draft/preview/publish webhook | editors in `/admin` |
| Кабинеты | та же доменная модель, отдельные user/project records | authenticated dynamic routes | RBAC в `/account` |

## Deployment

- CDN/edge cache для статических страниц и изображений;
- preview deployment на каждый change/PR;
- production с immutable assets, security headers и redirect map;
- секреты только в environment/secret store;
- rollback на предыдущий успешный deployment.

## Наблюдаемость

Ошибки build/runtime, Web Vitals, uptime критических маршрутов и анонимизированные conversion events. Собирать минимум данных и не загружать тяжёлую аналитику до интерактивности.

## Definition of done v1

Спецификации проходят review; выбранная Figma-концепция покрывает responsive/theme; build/typecheck/test/axe/Lighthouse проходят; страницы индексируемы; контент и медиа имеют права на публикацию; настроены production domain, sitemap, monitoring и rollback.
