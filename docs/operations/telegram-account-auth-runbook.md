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
| `TELEGRAM_UPDATE_MODE` | `polling` в production |
| `TELEGRAM_PROXY_URL` | внешний HTTP-прокси с авторизацией по IPv4 либо закрытый SSH-туннель `http://telegram-proxy:19840` |
| `TELEGRAM_PROXY_SSH_*`, `TELEGRAM_PROXY_REMOTE_*` | закрытый SSH forward до Squid |
| `ACCOUNT_TOKEN_HMAC_SECRET` | purpose-separated state/reset token hashes |
| `OUTBOX_ENCRYPTION_KEY` | шифрование payload до отправки |

Токен бота хранится в GitHub Actions Secrets. Production workflow создаёт
`/etc/arhdesign/production.env` на ВМ. Токен и приватный ключ следует передавать
только через предусмотренное хранилище секретов.

## Deployment verification

При старте poller вызывает `deleteWebhook` без удаления pending updates, затем
получает только `message` через `getUpdates`. Healthcheck прокси проверяет HTTPS
до `api.telegram.org` через туннель и ожидает ответ `2xx` или `3xx`. После запуска
production workflow вызывает `getMe` тем же маршрутом с токеном приложения
и ожидает `200`.

На production-ВМ с SSH-туннелем можно повторить обе проверки и посмотреть логи:

```sh
cd /opt/arhdesign
arh_compose() {
  docker compose --project-name arhdesign --profile telegram-proxy \
    --env-file /etc/arhdesign/production.env -f infra/docker-compose.yml "$@"
}
arh_compose ps telegram-proxy api
arh_compose exec -T telegram-proxy /usr/local/bin/telegram-proxy-check
arh_compose exec -T api sh -c 'printf "%s\n" "$TELEGRAM_BOT_TOKEN"' |
  arh_compose exec -T telegram-proxy /usr/local/bin/telegram-proxy-check --bot
arh_compose logs --since 20m --tail 100 telegram-proxy
arh_compose logs --since 20m --tail 200 api | grep 'Telegram polling'
```

Обе команды проверки завершаются без вывода при успехе. На старом релизе
`telegram-proxy-check` отсутствует. После успешного `getMe` проверьте `/start`
из тестового приватного чата, чтобы подтвердить получение обновлений и ответ бота.

## Замена SSH-узла

Если прежний узел недоступен, используйте доверенный SSH-сервер с доступом к
закрытому Squid. Squid должен разрешать HTTPS `CONNECT` к `api.telegram.org:443`,
а SSH-учётная запись должна иметь право пробрасывать порт до выбранного Squid.

1. В GitHub Actions Variables задайте `TELEGRAM_PROXY_SSH_HOST`,
   `TELEGRAM_PROXY_SSH_PORT`, `TELEGRAM_PROXY_SSH_USER`,
   `TELEGRAM_PROXY_REMOTE_HOST` и `TELEGRAM_PROXY_REMOTE_PORT` для нового маршрута.
   Пустые значения сохраняют прежние параметры.
2. На production-ВМ подготовьте ключ туннеля и проверенный ключ нового хоста в
   `/etc/arhdesign/telegram-proxy/id_ed25519` и `/etc/arhdesign/telegram-proxy/known_hosts`.
   Сверьте отпечаток сервера по доверенному каналу перед обновлением `known_hosts`.
3. Выпустите изменение штатным релизом по тегу с `main`. После деплоя проверьте
   `getMe` и `/start` из тестового чата.

## Готовый HTTP-прокси

Укажите адрес `http://host:port` в GitHub Actions Variable `TELEGRAM_PROXY_URL`.
Добавьте фактический исходящий IPv4 контейнера API в список разрешённых у
провайдера. Для внешнего адреса релиз запускает API без контейнера SSH-туннеля.
Smoke test выполняет `getMe` через заданный адрес из сети API. Пустое значение
переменной включает штатный SSH-туннель.

## Диагностика

| Симптом | Проверка | Действие |
| --- | --- | --- |
| SSH не подключается | логи `telegram-proxy` | проверить доступ с production-ВМ, ключ хоста и авторизацию SSH |
| SSH подключён, проверка HTTPS падает | логи `telegram-proxy`, Docker healthcheck | проверить Squid, разрешение `CONNECT`, DNS и исходящий HTTPS на SSH-узле |
| `getMe` с токеном возвращает `401` | результат проверки `--bot` | сверить `TELEGRAM_BOT_TOKEN` в GitHub Actions Secrets |
| `getMe` успешен, `/start` молчит | логи polling, в частности `409` | при конфликте найти второй экземпляр poller или активный webhook. После восстановления канала API продолжит polling |
| Кнопка открывает не того бота | `TELEGRAM_BOT_USERNAME` и frontend deep link | установить имя без `@`, пересобрать web |
| Бот не принимает ввод | private chat, state TTL, rate limit | начать `/start register` заново; не отключать limit |
| Пароль верный, профиль не активирован | account status, Argon2 comparison category, binding uniqueness | проверить allowlisted audit/result; не читать/логировать пароль |
| Reset не приходит | active binding, recipient status, outbox state/retry | исправить binding/delivery; не создавать ссылку вручную |

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
