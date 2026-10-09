# Unified account notification verification — 2026-09-30

## Scope

- add and remove chat participants;
- soft-delete a chat;
- desktop/tablet master-detail and mobile list/detail navigation;
- per-chat unread badges and read markers;
- one global cross-project notification center for chats, tasks, meetings,
  materials, finances, members and project documents;
- event-specific navigation to tasks, calendar, materials, finances, documents
  and the relevant chat;
- transactional in-app and Telegram notification creation for document and
  chat lifecycle changes;
- existing project, calendar, global chat, technical support, registration and Telegram flows.

## Results

| Check | Result | Time |
| --- | ---: | ---: |
| Go backend packages (`go test ./...`) | passed | 6.1 s |
| Frontend Vitest suite | 153 / 153 passed | 7.8 s |
| TypeScript (`tsc --noEmit`) | passed | — |
| ESLint | passed | — |
| Docker production build | passed | web 12.8 s; API 8.5 s |
| Cypress full suite | 40 / 40 passed | 39 s |
| Focused unified-notification Cypress spec | 5 / 5 passed | 2 s |
| Repeated local migration run | passed | — |
| Local Docker health | API and web healthy | — |

The focused Cypress collaboration spec contains five scenarios: named chat
creation, participant management and chat deletion, mobile navigation, a common
notification inbox containing task plus document-upload/document-delete events,
and project document upload/delete regression coverage.

## Environment

- Next.js 16.3.4;
- Cypress 15.3.0, Electron 136;
- Node.js 24.12.0;
- Go 1.25;
- PostgreSQL 17 in local Docker.

## Coverage gate

The frontend coverage command completed all 153 tests but correctly failed the
configured 90% gate: statements 83.76%, branches 75.63%, functions 82.85%,
lines 91.87%. The unified notification component itself is included in this
report, but the repository-wide shortfall is distributed across pre-existing
routes and project pages. Backend coverage was not declared: the coverage script
requires an explicit `TEST_DATABASE_URL`; ordinary and repository Go tests did
run successfully against the configured local integration environment.
