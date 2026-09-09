## Architecture

```text
Browser → Caddy (TLS) → Next.js web
                       → Go API → PostgreSQL
                                  → SMTP adapter / Telegram Bot adapter
```

- `apps/web`: Next.js App Router, TypeScript, FSD, semantic tokens Light/Dark.
- `apps/api`: Go REST API, `handler → service → repository`, contract-first OpenAPI.
- `openapi/`: единственный источник REST contract и generated types.
- `infra/`: Docker Compose, Caddy, env examples, backup/restore and rollback docs.

## Approved technology stack

| Area | Technology and rule |
| --- | --- |
| Web runtime | Next.js App Router, React Server Components by default, TypeScript with strict mode. |
| UI | Tailwind CSS only; semantic CSS variables for Light/Dark; `clsx`, `tailwind-merge` and `class-variance-authority` only inside reusable UI components when they reduce safe variant composition. |
| Forms and validation | React Hook Form + Zod in the browser; the Go API repeats all validation and remains authoritative. |
| API calls | Native `fetch` only, wrapped in `src/shared/api`; TypeScript request/response types are generated from `openapi/`. Axios is not used. Static server-rendered content uses Next.js server-side `fetch`; client mutations use the same typed transport. |
| Server/API | Go REST API, OpenAPI, generated Go/TypeScript contract types, PostgreSQL. |
| Tests | Vitest + React Testing Library, Playwright and axe-core; Go standard testing, `go vet` and integration tests against isolated PostgreSQL. |
| Helpers | Do not add the full `lodash` dependency. Prefer standard TypeScript/JavaScript and small local pure helpers in `src/shared/lib`. A focused dependency may be added only with a documented need and review approval. |

TanStack Query is not added in v1: the portfolio is mainly server-rendered and has one client mutation. It may be proposed in a separate change if authenticated or cache-heavy client data appears.

## Test coverage policy

Production frontend and Go backend code require at least **90%** coverage. The metric covers authored production source only: generated OpenAPI code, framework configuration, type declarations, migrations and test fixtures are excluded. E2E and visual tests are mandatory for their scenarios but do not replace unit/component coverage.

- Frontend: Vitest V8 coverage enforces 90% global thresholds for `src/` and reports `text`, `json` and `lcov` output.
- Go: the quality gate runs `go test -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...` and rejects total statement coverage below 90%. Go's standard coverage tooling does not calculate branch coverage; table-driven tests must explicitly cover every decision branch during review.
- A change may not lower an existing coverage value. Any narrowly justified exclusion must be documented in the relevant OpenSpec task and accepted in review.

## Frontend composition

Весь production UI реализуется Tailwind utility classes. Semantic CSS variables обеспечивают Light/Dark, а Tailwind theme — единые typography, spacing, radius и breakpoints. Повторно используемые примитивы располагаются в `src/shared/ui/` и импортируются через public API; feature/widget/page не создают локальные дубли Button, Input, Dialog, IconButton или CarouselControl.

## Contact delivery

Форма отправляет POST только в свой API. API валидирует данные, проверяет honeypot/rate limit, сохраняет submission в PostgreSQL и передаёт его через adapters:

- email на публичный рабочий адрес из env;
- Telegram Bot API в chat/channel из env.

Ошибка одного notification adapter не удаляет заявку из базы; пользователь получает нейтральный success только после сохранения заявки. PII не логируется целиком.

После успешного сохранения API создаёт cryptographically random anonymous cooldown token, сохраняет только его HMAC в PostgreSQL с `expires_at = created_at + 1 hour` и выставляет cookie `contact_cooldown` с `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, `Max-Age=3600`. Пока cookie и неистёкшая server-side запись совпадают, повторный `POST` формы не создаёт заявку, возвращает `429 Too Many Requests` и `Retry-After` с оставшимся числом секунд. UI показывает cooldown-state с временем до следующей отправки.

Это ограничение действует для того же браузера и не использует fingerprinting, localStorage или хранение PII ради идентификации. Очистка cookie или другой браузер считаются новой анонимной сессией; существующий IP rate limit и honeypot продолжают защищать endpoint от злоупотреблений.

## Authentication and token policy

### Public portfolio and contact form (v1)

Посетители не регистрируются и не проходят авторизацию. Landing, catalog, project pages и `POST` contact submission не требуют access/refresh tokens. Форма остаётся анонимной и защищается server validation, rate limit, honeypot и Origin/CORS policy; frontend не хранит никаких authentication tokens.

### Reserved model for future operator API

Защищённые operator endpoints (просмотр заявок, контент и настройки) добавляются только отдельным approved change. Для них применяется cookie-based session model на том же origin:

| Item | Rule |
| --- | --- |
| Access token | Short-lived signed JWT, audience and issuer checked, lifetime **15 minutes**. Stored only in `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/` cookie. |
| Refresh token | Cryptographically random opaque token, single-use rotation on every refresh, lifetime **30 days** of inactivity; maximum session lifetime **90 days**. Stored only in `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/` cookie. |
| Server storage | PostgreSQL stores only a slow/peppered hash of the refresh token plus session id, user id, expiry, rotation family and revocation timestamp. Raw refresh tokens, JWT signing keys and passwords are never stored in the database or logs. |
| Client storage | Never use `localStorage`, `sessionStorage`, IndexedDB, JavaScript variables persisted across navigation, URLs, query parameters or analytics for access/refresh tokens. Cookies are sent only over HTTPS in production. |
| Refresh and logout | Reuse of a rotated refresh token revokes its entire token family. Logout revokes the session and clears both cookies. Password change or operator deactivation revokes all sessions of that operator. |
| CSRF and requests | Any state-changing protected endpoint checks `Origin` and a synchronizer CSRF token. CORS is deny-by-default and accepts only configured first-party origins. |
| Credentials | Operator passwords are hashed with Argon2id; there is no public self-registration. MFA is required before exposing an operator UI to the internet. |

Token TTL values, cookie attributes and the implementation design must be repeated in OpenAPI security schemes and security tests when the protected API is introduced.

## Social links

Иконки VK и Telegram во всех применимых Header/Footer/Profile местах ведут во внешние сервисы в новой вкладке с `rel="noopener noreferrer"`:

- VK: `https://vk.ru/studio_architecture_design`;
- Telegram: `https://t.me/studio_architecture_design`.

## Design boundary

Утверждённый Figma Concept A — единственный источник визуального поведения. Design gate пройден: Desktop/Tablet/Mobile × Light/Dark согласованы. Если нужен новый UI state, сначала обновить Figma и получить approval.
