# R1 UI/QA scenario matrix

Статус: обязательный gate. Все достижимые ветки пользовательской логики R1
должны иметь Cypress-проверку. Unit/component-тесты дополняют, но не заменяют
проверку пути в браузере.

| Сценарий | UI-состояния | Cypress |
| --- | --- | --- |
| Хедер → модальная авторизация → успешный вход | modal, initial focus, pending, success | `cypress/e2e/telegram-registration.cy.js` |
| Мобильное меню → модальная авторизация | responsive navigation, initial focus, close | `cypress/e2e/telegram-registration.cy.js` |
| Регистрация → выбор роли | branded combobox, keyboard navigation, selected state | `cypress/e2e/telegram-registration.cy.js` |
| Авторизация → переключение на регистрацию | modal mode switch, регистрационная форма без отдельной страницы | `cypress/e2e/telegram-registration.cy.js` |
| Регистрация → выбор Telegram | form, pending, channel choice, переход в bot | `cypress/e2e/telegram-registration.cy.js` |
| Регистрация → выбор email | form, channel choice, mail accepted | `cypress/e2e/telegram-registration.cy.js` |
| Дубликат login/email | сохранённый draft, server error | `cypress/e2e/telegram-registration.cy.js` |
| Несовпадение паролей | client validation, запрос не отправлен | `cypress/e2e/telegram-registration.cy.js` |
| Telegram handoff без URL | fallback, повтор, переход на email | `cypress/e2e/telegram-registration.cy.js` |
| Credentials Telegram | отсутствие полей на сайте; ввод только в private bot | web: `cypress/e2e/telegram-registration.cy.js`; bot transport: Go handler/integration tests |
| Глобальный чат → отправка → вход второго пользователя | inbox, conversation, optimistic result, unread/read, CSRF, idempotency | `cypress/e2e/account-chats-technical-support.cy.js` |
| Техническая обратная связь | modal, complaint/suggestion, privacy warning, pending, Telegram-accepted success | `cypress/e2e/account-chats-technical-support.cy.js` |
| Ошибка frontend | dedupe в пределах вкладки, безопасный path, redaction, rate limit | Vitest component/API + Go service/integration; недетерминированный browser crash не эмулируется Cypress |
| Ошибка API | только `5xx`/panic, исходный ответ не зависит от alert delivery, без body/query/cookie | Go middleware/service/integration tests |
| Технический администратор | startup bootstrap, повтор без ротации пароля, вход, единственность активной роли, Telegram-only recipient | PostgreSQL integration tests |

Для следующих экранов R1 строки добавляются одновременно с реализацией. PR не
проходит QA, если изменённый интерактивный сценарий отсутствует в этой матрице
или соответствующий Cypress spec не выполняется.

## Отчётность

Команда `npm run test:qa` формирует два независимых доказательства:

1. `coverage/` — покрытие исполняемого frontend-кода строками и ветвями Vitest;
2. `reports/cypress/results.json` и `reports/cypress/summary.md` — покрытие
   исполнения перечисленных UI-сценариев и время их выполнения.

Покрытие исполнения равно отношению выполненных Cypress-тестов к обнаруженным.
Оно подтверждает отсутствие пропущенных/не запущенных тестов, но не доказывает,
что в матрице перечислена вся бизнес-логика. Поэтому изменение пользовательского
пути требует одновременно обновить эту матрицу и соответствующий spec.
