# Telegram account confirmation and recovery runbook

Статус: действующий для R1. Production bot: `@polismakovaSvetlanaBot`.

## Пользовательские сценарии

### Подтверждение регистрации

1. После регистрации сайт предлагает почту или Telegram.
2. Кнопка Telegram открывает
   `https://t.me/polismakovaSvetlanaBot?start=register`; полей Telegram-login на
   сайте нет.
3. Только в приватном чате бот просит логин сайта, затем пароль сайта.
4. State живёт не более 5 минут. Корректные данные атомарно активируют pending
   account и связывают private `chat_id`; email остаётся неподтверждённым.
5. Бот пытается удалить password message после успеха и ошибки. Пароль не
   сохраняется в БД/outbox/log/metric.

### Восстановление доступа

- Email можно выбрать только для подтверждённого адреса.
- Telegram можно выбрать только после существующей активной binding.
- Публичный ответ одинаков для неизвестного пользователя, неподтверждённого
  email и отсутствующей Telegram binding.
- Одноразовая reset-ссылка уходит в выбранный подтверждённый канал; после смены
  пароля старые sessions отзываются.

## Runtime secrets

| Secret/config | Назначение |
| --- | --- |
| `TELEGRAM_BOT_TOKEN` | Bot API; никогда не логируется |
| `TELEGRAM_BOT_USERNAME` | `polismakovaSvetlanaBot`, без `@` |
| `TELEGRAM_WEBHOOK_HOST` | hostname публичного webhook |
| `TELEGRAM_WEBHOOK_SECRET` | проверка `X-Telegram-Bot-Api-Secret-Token` |
| `TELEGRAM_RELAY_URL`, `TELEGRAM_RELAY_SECRET` | исходящая relay-доставка, если включена |
| `ACCOUNT_TOKEN_HMAC_SECRET` | purpose-separated state/reset token hashes |
| `OUTBOX_ENCRYPTION_KEY` | шифрование payload до отправки |

Значения находятся в GitHub Repository secrets и runtime `.env` с mode `600`.
Token/secret не передают в issue, Telegram, скриншот или shell command line.

## Deployment verification

Release workflow вызывает `setWebhook` с `allowed_updates=["message"]`, задаёт
команды `/start` и `/menu`, затем сравнивает URL из `getWebhookInfo` с ожидаемым.
Проверка оператора после deploy:

```sh
set -a
. /opt/arhdesign/.env
set +a
curl --fail --silent --show-error \
  "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getWebhookInfo" |
  jq '{ok, url: .result.url, pending: .result.pending_update_count,
       last_error_date: .result.last_error_date,
       last_error_message: .result.last_error_message}'
```

Вывод не содержит token. Ожидается правильный HTTPS URL, отсутствие
`last_error_message` и уменьшающийся `pending_update_count`.

## Диагностика

| Симптом | Проверка | Действие |
| --- | --- | --- |
| `/start` молчит | webhook URL/error, DNS/TLS, API health | исправить endpoint/TLS, повторить release webhook step |
| Кнопка открывает не того бота | `TELEGRAM_BOT_USERNAME` и frontend deep link | установить имя без `@`, пересобрать web |
| Бот не принимает ввод | private chat, state TTL, rate limit | начать `/start register` заново; не отключать limit |
| Пароль верный, профиль не активирован | account status, Argon2 comparison category, binding uniqueness | проверить allowlisted audit/result; не читать/логировать пароль |
| Reset не приходит | active binding, recipient status, outbox state/retry | исправить binding/delivery; не создавать ссылку вручную |
| `pending_update_count` растёт | webhook `5xx`, timeout, secret mismatch | проверить API/Caddy logs без payload, сверить secret rotation |

Для проверки связки используется отдельный тестовый account и приватный chat.
Production credentials пользователя в тест не копируются.

## Fallback и ручное восстановление

1. Если Telegram недоступен, пользователь выбирает подтверждение/восстановление
   по подтверждённой почте.
2. Если письмо задержано, разрешён resend после cooldown; проверяются SMTP,
   spam и DMARC, но существующий одноразовый token не показывается оператору.
3. Если доступен Telegram, но истёк state, пользователь снова запускает
   `/start register`; старый state использовать нельзя.
4. Если недоступны оба подтверждённых канала, R1 **не разрешает** оператору
   вручную активировать account, сообщить reset URL или изменить credential в
   SQL. Обращение регистрируется как security incident, личность проверяется
   вне системы по согласованной процедуре оператора. До появления отдельного
   аудируемого admin recovery flow доступ восстанавливается только после возврата
   контроля над одним из подтверждённых каналов.
5. При подозрении на захват account супер-администратор отключает пользователя,
   что отзывает sessions. Повторное включение не заменяет подтверждение владения
   каналом и не сбрасывает пароль.

Такой отказ от ad-hoc SQL является частью защиты от социальной инженерии.

## Проверки перед релизом

- unit/backend: private chat, TTL, wrong/correct credentials, race, deleteMessage,
  no password persistence/logging;
- PostgreSQL: одна active binding на user/chat и атомарная activation;
- Cypress: Telegram handoff, linked-channel reset, expired/rate-limited fallback,
  отсутствие Telegram credential fields на сайте;
- production smoke: `/start`, ошибочный пароль, повторный `/start`, тестовая
  binding и одно тестовое уведомление без PII.
