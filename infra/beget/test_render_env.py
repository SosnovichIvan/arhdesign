from __future__ import annotations

import importlib.util
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


MODULE_PATH = Path(__file__).with_name("render-env.py")
SPEC = importlib.util.spec_from_file_location("render_env", MODULE_PATH)
assert SPEC and SPEC.loader
render_env = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(render_env)


BASE_ENV = {
    "ACCOUNT_TOKEN_HMAC_SECRET": "a" * 32,
    "ADMIN_PASSWORD": "admin-password",
    "ADMIN_USERNAME": "admin",
    "CADDY_SITE": "designer-svetlana.ru",
    "COOLDOWN_HMAC_SECRET": "c" * 32,
    "EMAIL_FROM": "no-reply@example.test",
    "EMAIL_TO": "owner@example.test",
    "NEXT_PUBLIC_SITE_URL": "https://designer-svetlana.ru",
    "OUTBOX_ENCRYPTION_KEY": "BwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwc",
    "POSTGRES_DB": "arhdesign",
    "POSTGRES_PASSWORD": "p@ss:/ word",
    "POSTGRES_USER": "arhdesign",
    "SMTP_ADDRESS": "smtp.example.test:465",
    "SMTP_PASSWORD": "smtp-password",
    "SMTP_USERNAME": "no-reply@example.test",
    "TECH_ADMIN_EMAIL": "tech@example.test",
    "TECH_ADMIN_PASSWORD": "tech-password",
    "TECH_ADMIN_USERNAME": "tech",
    "TELEGRAM_BOT_TOKEN": "123:test",
    "TELEGRAM_BOT_USERNAME": "test_bot",
    "TELEGRAM_WEBHOOK_SECRET": "webhook-secret",
}


class RenderEnvironmentTest(unittest.TestCase):
    def test_api_uses_private_database_and_polling_proxy(self) -> None:
        with patch.dict(os.environ, BASE_ENV, clear=True):
            values = render_env.values_for("api", "sha", "1.2.3")
        self.assertEqual(values["TELEGRAM_UPDATE_MODE"], "polling")
        self.assertEqual(values["TELEGRAM_PROXY_REMOTE_HOST"], "10.66.66.1")
        self.assertIn("@10.16.0.30:5432/", values["DATABASE_URL"])
        self.assertIn("p%40ss%3A%2F%20word", values["DATABASE_URL"])

    def test_data_binds_only_private_address_and_starts_local_backup(self) -> None:
        with patch.dict(os.environ, BASE_ENV, clear=True):
            values = render_env.values_for("data", "sha", "1.2.3")
        self.assertEqual(values["DATA_PRIVATE_ADDRESS"], "10.16.0.30")
        self.assertEqual(values["OFFSITE_BACKUP_ENABLED"], "false")
        self.assertEqual(values["BACKUP_INTERVAL_SECONDS"], "86400")

    def test_rejects_multiline_secret(self) -> None:
        invalid = dict(BASE_ENV, SMTP_PASSWORD="first\nsecond")
        with patch.dict(os.environ, invalid, clear=True):
            with self.assertRaises(SystemExit):
                render_env.values_for("api", "sha", "1.2.3")

    def test_written_file_is_owner_only_and_dotenv_quoted(self) -> None:
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, BASE_ENV, clear=True):
            output = Path(directory, "edge.env")
            values = render_env.values_for("edge", "sha", "1.2.3")
            output.write_text(
                "".join(f"{key}={render_env.json.dumps(value)}\n" for key, value in values.items()),
                encoding="utf-8",
            )
            output.chmod(0o600)
            self.assertEqual(output.stat().st_mode & 0o777, 0o600)
            self.assertIn('CADDY_SITE="designer-svetlana.ru"', output.read_text())


if __name__ == "__main__":
    unittest.main()
