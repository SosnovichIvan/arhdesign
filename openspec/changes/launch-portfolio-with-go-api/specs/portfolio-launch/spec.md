## ADDED Requirements

### Requirement: Собственная контактная форма

Сайт MUST отправлять заявки только в собственный Go API и сохранять их до доставки уведомлений.

#### Scenario: Успешная заявка

- **WHEN** посетитель заполняет валидную форму и отправляет её
- **THEN** API сохраняет заявку, UI показывает success state, а notification adapters получают данные без раскрытия секретов клиенту.

### Requirement: Защита контактного endpoint

Endpoint заявок MUST валидировать входные данные и защищаться от автоматизированных повторных отправок.

#### Scenario: Невалидная или подозрительная заявка

- **WHEN** request не проходит validation, rate limit или honeypot
- **THEN** API не создаёт обычную заявку и возвращает согласованный безопасный response.

### Requirement: One-hour cooldown после успешной заявки

После успешного сохранения contact submission API MUST записывать анонимный cooldown для текущего браузера на один час. До истечения cooldown повторная отправка MUST не создавать новую заявку, а frontend MUST показать время ожидания.

#### Scenario: Повторная отправка до истечения часа

- **WHEN** тот же браузер отправляет форму повторно до истечения одного часа после успешной заявки
- **THEN** API возвращает `429 Too Many Requests` с `Retry-After`, не создаёт новую submission и не вызывает notification adapters, а UI показывает cooldown-state.

#### Scenario: Отправка после истечения cooldown

- **WHEN** истекли 60 минут с момента успешной заявки
- **THEN** тот же браузер может отправить новую валидную форму обычным contact flow.

### Requirement: Публичные endpoints не используют авторизацию

Страницы портфолио и contact submission endpoint MUST быть доступны без visitor account, access token или refresh token. Они MUST не создавать и не хранить authentication token в браузере.

#### Scenario: Анонимная отправка формы

- **WHEN** посетитель открывает сайт и отправляет валидную форму
- **THEN** request проходит защиту от спама без Authorization header или authentication cookie, а frontend не записывает token в browser storage.

### Requirement: Модель будущей операторской авторизации

Защищённые operator endpoints, если они будут добавлены отдельным approved change, MUST использовать 15-minute access JWT и rotating opaque refresh token с idle TTL 30 days и absolute TTL 90 days. Оба токена MUST передаваться только в `HttpOnly`, `Secure` cookies; refresh token MUST храниться на сервере только в hashed form.

#### Scenario: Повторно использованный refresh token

- **WHEN** API получает уже использованный refresh token
- **THEN** API отзывает всю token family и требует повторной аутентификации, не раскрывая токен или персональные данные в response/logs.

### Requirement: Внешние социальные ссылки

Все утверждённые VK и Telegram controls MUST вести на предоставленные публичные профили.

#### Scenario: Переход в VK

- **WHEN** посетитель нажимает иконку VK
- **THEN** открывается `https://vk.ru/studio_architecture_design` в новой вкладке без доступа новой страницы к исходному окну.

#### Scenario: Переход в Telegram

- **WHEN** посетитель нажимает иконку Telegram
- **THEN** открывается `https://t.me/studio_architecture_design` в новой вкладке без доступа новой страницы к исходному окну.

### Requirement: VDS-ready runtime

Приложение MUST запускаться через Docker Compose с public reverse proxy, внутренними API/database services и health checks.

#### Scenario: Локальный production-like запуск

- **WHEN** оператор запускает documented Compose workflow
- **THEN** web и API доступны через proxy, PostgreSQL не опубликован наружу, а health checks показывают готовность сервисов.

### Requirement: Minimum code coverage

Authored production frontend code MUST maintain at least 90% coverage for lines, functions, branches and statements. Authored Go backend code MUST maintain at least 90% total statement coverage using Go atomic coverage; table-driven tests MUST cover every decision branch because Go standard coverage does not provide a branch metric. Generated code, framework configuration, type declarations, migrations and test fixtures MUST be excluded from the metric.

#### Scenario: Coverage below threshold

- **WHEN** a frontend or backend coverage report is below 90% for any required metric
- **THEN** the relevant local quality command and CI quality gate fail.
