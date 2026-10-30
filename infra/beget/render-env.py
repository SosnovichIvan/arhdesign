#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
from urllib.parse import quote


def required(name: str) -> str:
    value = os.environ.get(name, "")
    if not value or "\n" in value or "\r" in value:
        raise SystemExit(f"{name} is required and must be a single line")
    return value


def optional(name: str, default: str = "") -> str:
    value = os.environ.get(name, default)
    if "\n" in value or "\r" in value:
        raise SystemExit(f"{name} must be a single line")
    return value


def database_url() -> str:
    user = quote(required("POSTGRES_USER"), safe="")
    password = quote(required("POSTGRES_PASSWORD"), safe="")
    database = quote(required("POSTGRES_DB"), safe="")
    return f"postgres://{user}:{password}@10.16.0.30:5432/{database}?sslmode=disable"


def common(release_tag: str, release_version: str) -> dict[str, str]:
    return {
        "RELEASE_IMAGE_TAG": release_tag,
        "RELEASE_VERSION": release_version,
    }


def values_for(role: str, release_tag: str, release_version: str) -> dict[str, str]:
    values = common(release_tag, release_version)
    if role == "edge":
        values.update(
            CADDY_SITE=required("CADDY_SITE"),
            API_PRIVATE_ADDRESS="10.16.0.20",
        )
        return values
    if role == "data":
        values.update(
            DATA_ENV_FILE="/etc/arhdesign/data.env",
            DATA_PRIVATE_ADDRESS="10.16.0.30",
            POSTGRES_DB=required("POSTGRES_DB"),
            POSTGRES_USER=required("POSTGRES_USER"),
            POSTGRES_PASSWORD=required("POSTGRES_PASSWORD"),
            BACKUP_INTERVAL_SECONDS="86400",
            BACKUP_LOCAL_RETENTION_DAYS="7",
            OFFSITE_BACKUP_ENABLED="false",
        )
        return values
    values.update(
        API_ENV_FILE="/etc/arhdesign/api.env",
        API_PRIVATE_ADDRESS="10.16.0.20",
        DATABASE_URL=database_url(),
        PUBLIC_ORIGIN=required("NEXT_PUBLIC_SITE_URL"),
        CONTACT_RETENTION_DAYS="365",
        ACCOUNT_AUTH_ENABLED="true",
        ACCOUNT_TOKEN_HMAC_SECRET=required("ACCOUNT_TOKEN_HMAC_SECRET"),
        ACCOUNT_TOKEN_HMAC_KEY_VERSION="1",
        OUTBOX_ENCRYPTION_KEY=required("OUTBOX_ENCRYPTION_KEY"),
        OUTBOX_ENCRYPTION_KEY_VERSION="1",
        COOLDOWN_HMAC_SECRET=required("COOLDOWN_HMAC_SECRET"),
        MONITORING_TIMEZONE="Europe/Moscow",
        MONITORING_REPORT_HOUR="9",
        SMTP_ADDRESS=required("SMTP_ADDRESS"),
        SMTP_USERNAME=required("SMTP_USERNAME"),
        SMTP_PASSWORD=required("SMTP_PASSWORD"),
        EMAIL_FROM=required("EMAIL_FROM"),
        EMAIL_TO=optional("EMAIL_TO"),
        TELEGRAM_BOT_TOKEN=required("TELEGRAM_BOT_TOKEN"),
        TELEGRAM_BOT_USERNAME=required("TELEGRAM_BOT_USERNAME"),
        TELEGRAM_WEBHOOK_SECRET=required("TELEGRAM_WEBHOOK_SECRET"),
        TELEGRAM_NOTIFICATION_RETENTION_HOURS="24",
        TELEGRAM_UPDATE_MODE="polling",
        TELEGRAM_PROXY_SSH_HOST="89.126.250.245",
        TELEGRAM_PROXY_SSH_PORT="22",
        TELEGRAM_PROXY_SSH_USER="root",
        TELEGRAM_PROXY_REMOTE_HOST="10.66.66.1",
        TELEGRAM_PROXY_REMOTE_PORT="3128",
        TELEGRAM_PROXY_IDENTITY_FILE="/etc/arhdesign/telegram-proxy/id_ed25519",
        TELEGRAM_PROXY_KNOWN_HOSTS_FILE="/etc/arhdesign/telegram-proxy/known_hosts",
        ADMIN_USERNAME=required("ADMIN_USERNAME"),
        ADMIN_PASSWORD=required("ADMIN_PASSWORD"),
        TECH_ADMIN_USERNAME=required("TECH_ADMIN_USERNAME"),
        TECH_ADMIN_EMAIL=required("TECH_ADMIN_EMAIL"),
        TECH_ADMIN_PASSWORD=required("TECH_ADMIN_PASSWORD"),
    )
    return values


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("role", choices=("edge", "api", "data"))
    parser.add_argument("output", type=Path)
    parser.add_argument("release_tag")
    parser.add_argument("release_version")
    args = parser.parse_args()

    values = values_for(args.role, args.release_tag, args.release_version)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        "".join(f"{key}={json.dumps(value, ensure_ascii=False)}\n" for key, value in values.items()),
        encoding="utf-8",
    )
    args.output.chmod(0o600)


if __name__ == "__main__":
    main()
