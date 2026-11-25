#!/bin/sh
set -eu

url=https://api.telegram.org/
expected_status='[23][0-9][0-9]'
proxy_url=${2:-http://127.0.0.1:${TELEGRAM_PROXY_LOCAL_PORT:-19840}}
if [ "${1:-}" = "--bot" ]; then
  if ! IFS= read -r token; then
    echo "Telegram bot token was not provided" >&2
    exit 1
  fi
  case "$token" in
    ''|*[!a-zA-Z0-9:_-]*)
      echo "Invalid Telegram bot token format" >&2
      exit 1
      ;;
  esac
  url="https://api.telegram.org/bot${token}/getMe"
  expected_status=200
fi

# Keep the bot token out of the curl arguments and diagnostic output.
if status=$(printf 'url = "%s"\n' "$url" |
  curl --disable --config - --silent --output /dev/null --write-out '%{http_code}' \
    --proxy "$proxy_url" --noproxy '' \
    --connect-timeout 3 --max-time 6); then
  case "$status" in
    $expected_status) exit 0 ;;
  esac
  echo "Telegram HTTPS probe returned HTTP $status" >&2
else
  code=$?
  echo "Telegram HTTPS probe through the proxy failed (curl exit $code)" >&2
fi
exit 1
