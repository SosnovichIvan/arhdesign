#!/bin/sh
set -eu

role="${1:-}"
case "$role" in
  edge|api|data) ;;
  *) echo "usage: $0 edge|api|data" >&2; exit 64 ;;
esac

if [ "$role" = data ]; then
  systemctl disable --now arhdesign-data-egress.service 2>/dev/null || true
  rm -f /etc/systemd/system/arhdesign-data-egress.service
  ip route del default via 10.16.0.20 dev eth1 2>/dev/null || true
  cat > /etc/apt/apt.conf.d/90arhdesign-proxy <<'EOF'
Acquire::http::Proxy "http://10.16.0.20:8888";
Acquire::https::Proxy "http://10.16.0.20:8888";
EOF
  export http_proxy=http://10.16.0.20:8888
  export https_proxy=http://10.16.0.20:8888
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y ca-certificates curl docker.io docker-compose-v2 ufw
if [ "$role" = api ]; then
  apt-get install -y tinyproxy
  cat > /etc/tinyproxy/tinyproxy.conf <<'EOF'
User tinyproxy
Group tinyproxy
Port 8888
Listen 10.16.0.20
Timeout 600
DefaultErrorFile "/usr/share/tinyproxy/default.html"
StatFile "/usr/share/tinyproxy/stats.html"
LogFile "/var/log/tinyproxy/tinyproxy.log"
LogLevel Notice
PidFile "/run/tinyproxy/tinyproxy.pid"
MaxClients 32
StartServers 2
MinSpareServers 2
MaxSpareServers 8
Allow 10.16.0.30
ConnectPort 443
EOF
  systemctl enable tinyproxy
  systemctl restart tinyproxy
  if grep -q '^# arhdesign-private-egress$' /etc/ufw/before.rules; then
    temporary_rules="$(mktemp)"
    tail -n +7 /etc/ufw/before.rules > "$temporary_rules"
    install -m 0640 "$temporary_rules" /etc/ufw/before.rules
    rm -f "$temporary_rules"
  fi
  while iptables -t nat -C POSTROUTING -s 10.16.0.30/32 -o eth0 -j MASQUERADE 2>/dev/null; do
    iptables -t nat -D POSTROUTING -s 10.16.0.30/32 -o eth0 -j MASQUERADE
  done
fi
systemctl enable --now docker

if [ "$role" = data ]; then
  install -d -m 0755 /etc/systemd/system/docker.service.d
  cat > /etc/systemd/system/docker.service.d/10-arhdesign-egress-proxy.conf <<'EOF'
[Service]
Environment="HTTP_PROXY=http://10.16.0.20:8888"
Environment="HTTPS_PROXY=http://10.16.0.20:8888"
Environment="NO_PROXY=127.0.0.1,localhost,10.16.0.0/16"
EOF
  systemctl daemon-reload
  systemctl restart docker
fi

install -d -m 0755 /opt/arhdesign
install -d -m 0700 /etc/arhdesign
install -d -m 0700 /etc/arhdesign/telegram-proxy

cat > /etc/ssh/sshd_config.d/00-arhdesign-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin prohibit-password
X11Forwarding no
AllowAgentForwarding no
EOF
rm -f /etc/ssh/sshd_config.d/99-arhdesign-hardening.conf
sshd -t
systemctl reload ssh

ufw default deny incoming
ufw default allow outgoing
case "$role" in
  edge)
    ufw allow 22/tcp
    ufw allow 80/tcp
    ufw allow 443/tcp
    ufw allow 443/udp
    ;;
  api)
    ufw allow 22/tcp
    ufw allow from 10.16.0.10 to any port 8080 proto tcp
    ufw allow from 10.16.0.30 to any port 8888 proto tcp
    ;;
  data)
    ufw allow from 10.16.0.20 to any port 22 proto tcp
    ufw allow from 10.16.0.20 to any port 5432 proto tcp
    ;;
esac

ufw --force enable
ufw --force reload

docker version >/dev/null
docker compose version >/dev/null
printf 'bootstrap complete: %s\n' "$role"
