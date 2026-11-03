#!/bin/sh
set -eu

deploy_user="${1:-arhadmin}"
if ! id "$deploy_user" >/dev/null 2>&1; then
  echo "deploy user does not exist: $deploy_user" >&2
  exit 64
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y ca-certificates curl docker.io docker-compose-v2 ufw
systemctl enable --now docker
usermod -aG docker "$deploy_user"

if ! getent group arhdesign >/dev/null 2>&1; then
  groupadd --system arhdesign
fi
usermod -aG arhdesign "$deploy_user"

install -d -m 0755 /opt/arhdesign
install -d -o root -g arhdesign -m 0750 /etc/arhdesign
install -d -o root -g arhdesign -m 0750 /etc/arhdesign/telegram-proxy
if [ -f /etc/arhdesign/telegram-proxy/id_ed25519 ]; then
  chown root:arhdesign /etc/arhdesign/telegram-proxy/id_ed25519
  chmod 0640 /etc/arhdesign/telegram-proxy/id_ed25519
fi
if [ -f /etc/arhdesign/telegram-proxy/known_hosts ]; then
  chown root:arhdesign /etc/arhdesign/telegram-proxy/known_hosts
  chmod 0640 /etc/arhdesign/telegram-proxy/known_hosts
fi
chown -R "$deploy_user:$deploy_user" /opt/arhdesign

cat > /etc/ssh/sshd_config.d/00-arhdesign-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
X11Forwarding no
AllowAgentForwarding no
EOF
sshd -t
systemctl reload ssh

ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 443/udp
ufw --force enable
ufw --force reload

docker version >/dev/null
docker compose version >/dev/null
printf 'bootstrap complete: yandex single-node\n'
