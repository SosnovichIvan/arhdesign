# Beget: три VPS для arhDesign

Статус: принято для первого production-развёртывания. Пересмотр: при устойчивой загрузке CPU выше 70%, нехватке памяти, заполнении NVMe выше 70% или переходе к отдельному файловому хранилищу.

## Топология и стоимость

| Узел | Роль | Конфигурация | Сеть | Стоимость |
| --- | --- | --- | --- | ---: |
| `arh-edge` | Caddy, TLS, Next.js | 2 CPU, 2 ГБ RAM, 30 ГБ NVMe | публичный IPv4 + private | 43,44 ₽/сутки |
| `arh-api` | Go API, workers, Telegram SSH tunnel | 2 CPU, 2 ГБ RAM, 30 ГБ NVMe | публичный IPv4 + private | 43,44 ₽/сутки |
| `arh-data` | PostgreSQL 17, документы, локальные backup snapshots | 2 CPU, 3 ГБ RAM, 50 ГБ NVMe | только private | 56,01 ₽/сутки |

Итого: 142,89 ₽/сутки. Каждый узел дешевле установленного лимита 60 ₽/сутки. ОС — Ubuntu 24.04 LTS. Диск data-узла можно только увеличивать, поэтому заполнение контролируется заранее.

Фактически выделенные адреса:

- `arh-edge`: public `193.176.78.228`, private `10.16.0.10`;
- `arh-api`: public `45.153.188.63`, private `10.16.0.20`;
- `arh-data`: private `10.16.0.30`, публичного адреса нет.

## Границы

- Единственная публичная точка приложения — `arh-edge`: порты 80/443.
- `arh-api` принимает приложение на private IP:8080; публичный IPv4 нужен для администрирования и исходящих соединений, входящий 8080 закрывается firewall.
- `arh-data` не имеет публичного IP. PostgreSQL слушает только private IP:5432 и принимает соединения только от private IP API.
- Beget не выдаёт default route узлу без публичного IP. Для `apt` и Docker pull `arh-data` использует Tinyproxy на `10.16.0.20:8888`; firewall API разрешает этот порт только источнику `10.16.0.30`. Приложение, PostgreSQL и пользовательский трафик через этот proxy не проходят.
- Проектные документы пока хранятся в PostgreSQL с лимитом 10 МиБ на объект; отдельное object storage не вводится до измеренной необходимости.
- Backup на том же data-узле не считается полноценной offsite-копией. Перед production требуется зашифрованная копия вне `arh-data` и проверка восстановления.

## Telegram без Cloudflare

API работает в режиме long polling. Все запросы Telegram — `deleteWebhook`, `getUpdates`, отправка сообщений и удаление временных сообщений — используют один HTTP client с `TELEGRAM_PROXY_URL=http://telegram-proxy:19840`.

Контейнер `telegram-proxy` поднимает закрытый SSH local-forward до Squid по образцу plannerBot. Прямого fallback нет: при недоступном туннеле Telegram временно не работает, API продолжает работу и повторяет доставку через outbox. SSH-ключ и `known_hosts` находятся только на сервере в `/etc/arhdesign/telegram-proxy`, не в Git и не в `.env`.

Ограничение на proxy-хосте: отдельный ключ, `restrict,permitopen="10.66.66.1:3128"`, без shell/PTY/agent/X11 forwarding. Squid не публикуется в интернет.

## Контракты развёртывания

- `infra/beget/compose-edge.yml` владеет edge-узлом.
- `infra/beget/compose-api.yml` владеет API и Telegram tunnel.
- `infra/beget/compose-data.yml` владеет PostgreSQL и backup worker.
- Образы выпускаются только тегом `MAJOR.MINOR.PATCH` на коммите из `main`.
- Секреты доставляются отдельно на целевой узел с правами `0600`; значения не выводятся в CI logs.
- Deploy SSH идёт по ключу: edge/API напрямую, data — через `ProxyJump arh-api`. Парольная SSH-аутентификация после bootstrap отключается.
- Порядок обновления: backup → data health → migration → API health → web/edge health → smoke tests.
