from __future__ import annotations

import importlib.util
import os
import runpy
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


MODULE_PATH = Path(__file__).with_name("render-env.py")
SPEC = importlib.util.spec_from_file_location("yandex_render_env", MODULE_PATH)
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
    def test_single_node_uses_internal_service_names_and_polling_proxy(self) -> None:
        with patch.dict(os.environ, BASE_ENV, clear=True):
            values = render_env.values("sha", "1.2.3")
        self.assertEqual(values["TELEGRAM_UPDATE_MODE"], "polling")
        self.assertEqual(values["TELEGRAM_PROXY_URL"], "http://telegram-proxy:19840")
        self.assertEqual(values["POSTGRES_SHARED_BUFFERS"], "768MB")
        self.assertEqual(values["OFFSITE_BACKUP_ENABLED"], "false")

    def test_rejects_multiline_secret(self) -> None:
        invalid = dict(BASE_ENV, SMTP_PASSWORD="first\nsecond")
        with patch.dict(os.environ, invalid, clear=True):
            with self.assertRaises(SystemExit):
                render_env.values("sha", "1.2.3")

    def test_rejects_multiline_optional_value(self) -> None:
        invalid = dict(BASE_ENV, EMAIL_TO="first@example.test\rsecond@example.test")
        with patch.dict(os.environ, invalid, clear=True):
            with self.assertRaises(SystemExit):
                render_env.values("sha", "1.2.3")

    def test_written_file_is_owner_only_and_dotenv_quoted(self) -> None:
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, BASE_ENV, clear=True):
            output = Path(directory, "production.env")
            with patch.object(sys, "argv", ["render-env.py", str(output), "sha", "1.2.3"]):
                render_env.main()
            self.assertEqual(output.stat().st_mode & 0o777, 0o600)
            contents = output.read_text()
            self.assertIn('CADDY_SITE="designer-svetlana.ru"', contents)
            self.assertIn('POSTGRES_PASSWORD="p@ss:/ word"', contents)

    def test_cli_entrypoint_writes_environment(self) -> None:
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, BASE_ENV, clear=True):
            output = Path(directory, "production.env")
            with patch.object(sys, "argv", ["render-env.py", str(output), "sha", "1.2.3"]):
                runpy.run_path(str(MODULE_PATH), run_name="__main__")
            self.assertIn('RELEASE_VERSION="1.2.3"', output.read_text())


if __name__ == "__main__":
    unittest.main()
