## ADDED Requirements

### Requirement: Регистрация с выбором канала подтверждения

Система MUST создавать учётную запись только с уникальными нормализованными
login/email, обязательным именем, password policy и выбранной профессиональной
ролью; после регистрации пользователь MUST выбрать подтверждение ссылкой через
email либо проверкой credentials в личном Telegram-чате. Доступ в кабинет MUST открываться только
после применения действующего email verification token либо успешной проверки
login/password в личном Telegram-чате. Telegram-вариант MUST открывать бота по
ссылке, последовательно запросить login и password, проверить credential через
тот же Identity service и атомарно активировать pending account вместе с
привязкой chat. Web-клиент MUST NOT показывать поля login/password для
Telegram-подтверждения и MUST NOT принимать эти credentials: нажатие выбора
Telegram сразу открывает приложение Telegram в личном чате с ботом; web-экран
остаётся только fallback для повторного открытия приложения или выбора email.

#### Scenario: Успешная регистрация и подтверждение

- **WHEN** посетитель отправляет валидные данные, выбирает email и затем применяет действующий verification token из письма
- **THEN** система атомарно создаёт профиль, подтверждает email, инвалидирует token и позволяет войти без повторного создания аккаунта.

#### Scenario: Подтверждение через Telegram

- **WHEN** pending пользователь выбирает Telegram, открывает бота по ссылке и вводит корректные login/password регистрации в личном чате
- **THEN** система атомарно активирует account, связывает chat с user, отзывает неиспользованные verification tokens и отвечает «Профиль подтверждён» без дополнительной ссылки.

#### Scenario: Неверные данные или небезопасный чат

- **WHEN** login/password неверны, account отсутствует/disabled либо команда вызвана в группе/канале
- **THEN** бот не активирует account, не связывает chat, не уточняет неверное поле и применяет общий rate limit для Telegram login.

#### Scenario: Обработка сообщения с password

- **WHEN** бот получает сообщение с password
- **THEN** сервис использует password только в памяти текущего запроса, не пишет его в БД/log/audit/outbox и немедленно запрашивает удаление исходного Telegram message независимо от результата проверки.

#### Scenario: Дубликат или недействительный token

- **WHEN** login/email уже занят либо token истёк, неверен или использован
- **THEN** система не создаёт дубликат и возвращает безопасное состояние без раскрытия security data.

### Requirement: Защищённая cookie-сессия

Система MUST аутентифицировать account routes через server-side revocable
session с HttpOnly/Secure/SameSite cookies, rotation, absolute/idle expiry,
Origin/CSRF validation и запретом хранения tokens в browser storage/URL/logs.

#### Scenario: Login, refresh и logout

- **WHEN** подтверждённый active пользователь вводит верный login/password
- **THEN** система фиксирует interactive login audit event, создаёт session, безопасно вращает refresh token и отзывает session при logout.

#### Scenario: Reuse или отключение пользователя

- **WHEN** повторно используется уже rotated token либо super-admin отключает пользователя
- **THEN** система отзывает затронутую family или все sessions и запрещает следующий protected request.

### Requirement: Восстановление пароля без account enumeration

Система MUST возвращать одинаковый forgot-password ответ для существующего и
несуществующего идентификатора и выбранного канала, хранить только hash
одноразового reset token и отзывать все sessions после успешной смены password.
Email-доставка MUST быть доступна только для подтверждённого email, а Telegram-
доставка — только для заранее связанного active private chat.

#### Scenario: Успешный reset

- **WHEN** пользователь применяет действующий single-use reset token и валидный новый password
- **THEN** credential заменяется, token инвалидируется, все sessions отзываются и создаётся audit event.

#### Scenario: Повторный или истёкший reset

- **WHEN** reset token истёк или уже использован
- **THEN** credential не меняется и token нельзя применить повторно.

#### Scenario: Восстановление через связанный Telegram

- **WHEN** пользователь выбирает Telegram и для account существует active verified private chat binding
- **THEN** бот отправляет одноразовую reset-ссылку в связанный chat, а публичный web-ответ остаётся таким же, как для отсутствующего account или недоступного канала.

#### Scenario: Telegram не был связан заранее

- **WHEN** выбран Telegram, но active verified chat binding отсутствует
- **THEN** система не предлагает вход по забытому password в боте, ничего не отправляет и возвращает тот же нейтральный web-ответ без account enumeration.

### Requirement: Управление пользователями супер-администратором

Система MUST разрешать только super-admin просматривать bounded список
пользователей и отключать/восстанавливать доступ, MUST отзывать sessions при
отключении и MUST сохранять хотя бы одного active super-admin.

#### Scenario: Отключение пользователя

- **WHEN** super-admin отключает active обычного пользователя
- **THEN** статус и audit фиксируются атомарно, sessions отзываются, новый login и protected requests запрещены.

#### Scenario: Последний супер-администратор

- **WHEN** запрошено отключение единственного active super-admin
- **THEN** система отклоняет операцию и не изменяет пользователя или sessions.

#### Scenario: Конкурентное отключение двух супер-администраторов

- **WHEN** два active super-admin одновременно пытаются отключить друг друга
- **THEN** операции сериализуются одним Identity guard, не более одной операции коммитится и в системе остаётся хотя бы один active super-admin.

### Requirement: Защита auth от перебора и безопасный аудит

Система MUST применять конкурентно-безопасные лимиты по HMAC-маркерам actor,
анонимного источника, account identifier или session family согласно
`docs/architecture/client-cabinet-r1-auth-abuse-audit-contract.md`; MUST
возвращать `429` с `Retry-After` без раскрытия account existence; MUST fail
closed для login/reset/admin mutation при недоступности limiter. Security audit
MUST принимать только allowlisted metadata и MUST NOT содержать password, raw
token/cookie, login, email или raw IP.

#### Scenario: Исчерпан лимит absent account

- **WHEN** посетитель исчерпал login или forgot-password bucket для несуществующего identifier
- **THEN** система возвращает тот же публичный `rate_limited` envelope, не создаёт/не изменяет account и не записывает submitted identifier в audit или logs.

#### Scenario: Limiter недоступен

- **WHEN** PostgreSQL limiter недоступен во время login, reset-password или admin mutation
- **THEN** система возвращает `503 service_unavailable`, не выполняет доменную операцию и не обходит лимит.

#### Scenario: Старые cookie после disable

- **WHEN** disable transaction закоммичена и пользователь предъявляет старую access или refresh cookie
- **THEN** система отклоняет запрос, не восстанавливает session, а restore account впоследствии всё равно требует нового login.
