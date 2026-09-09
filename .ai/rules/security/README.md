# Security

- Секреты, SMTP credentials, Telegram token и DB URL хранятся только в env/secret storage, никогда в git, логах или клиентском коде.
- Форма защищена серверной валидацией, rate limit, honeypot и origin/CORS policy. CAPTCHA добавляется отдельным approved change, если понадобится.
- В логах маскируются email, телефон и текст обращения; PII не попадает в trace/error payload.
- Reverse proxy завершает TLS; API доверяет forwarded headers только от этого proxy.
- Перед доставкой проверить `.env.example`, `.gitignore` и отсутствие секретов в diff.
- Публичные portfolio/contact endpoints v1 анонимны: не выпускать и не хранить visitor auth tokens. Будущие operator endpoints требуют отдельного approved OpenSpec change.
- После успешной anonymous contact submission устанавливать server-backed cooldown на 1 час: хранить только HMAC случайного cookie token и expiry, отвечать `429` + `Retry-After` до окончания периода. Не применять browser fingerprinting или PII для cooldown.
- Для operator API: access JWT — 15 минут в `HttpOnly`, `Secure`, `SameSite=Lax` cookie; opaque rotating refresh token — 30 дней inactivity, 90 дней absolute lifetime в `HttpOnly`, `Secure`, `SameSite=Strict` cookie. Не использовать token в `localStorage`, `sessionStorage`, URL, логах или аналитике.
- В БД хранить только peppered hash refresh token, session metadata и revocation state. Повторное использование refresh token отзывает всю token family; state-changing requests дополнительно защищать Origin и CSRF validation.
