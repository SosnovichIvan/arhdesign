#!/bin/sh
set -eu

: "${TELEGRAM_PROXY_SSH_HOST:?TELEGRAM_PROXY_SSH_HOST is required}"
: "${TELEGRAM_PROXY_SSH_USER:?TELEGRAM_PROXY_SSH_USER is required}"
: "${TELEGRAM_PROXY_REMOTE_HOST:?TELEGRAM_PROXY_REMOTE_HOST is required}"
: "${TELEGRAM_PROXY_REMOTE_PORT:?TELEGRAM_PROXY_REMOTE_PORT is required}"

install -m 0600 /run/secrets/telegram_proxy_identity /tmp/id_ed25519
install -m 0600 /run/secrets/telegram_proxy_known_hosts /tmp/known_hosts

exec ssh -NT \
  -p "${TELEGRAM_PROXY_SSH_PORT:-22}" \
  -i /tmp/id_ed25519 \
  -o BatchMode=yes \
  -o ConnectTimeout=15 \
  -o ExitOnForwardFailure=yes \
  -o IdentitiesOnly=yes \
  -o ServerAliveCountMax=3 \
  -o ServerAliveInterval=20 \
  -o StrictHostKeyChecking=yes \
  -o UserKnownHostsFile=/tmp/known_hosts \
  -L "0.0.0.0:${TELEGRAM_PROXY_LOCAL_PORT:-19840}:${TELEGRAM_PROXY_REMOTE_HOST}:${TELEGRAM_PROXY_REMOTE_PORT}" \
  "${TELEGRAM_PROXY_SSH_USER}@${TELEGRAM_PROXY_SSH_HOST}"
