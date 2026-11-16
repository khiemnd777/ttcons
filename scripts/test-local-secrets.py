#!/usr/bin/env python3
"""Isolated regression checks; never start Docker or use real credentials."""

import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parent.parent
spec = importlib.util.spec_from_file_location("local_secrets", ROOT / "scripts/check-local-secrets.py")
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)


class SecretChecks(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="consflow-secret-test-")
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.secret = self.base / "secret with spaces"
        self.secret.write_text("synthetic-test-value")
        self.secret.chmod(0o600)
        self.config = {
            "services": {"backend": {"secrets": [{"source": "zoho_smtp_password"}],
                "user": f"{os.getuid()}:{os.getgid()}"}},
            "secrets": {"zoho_smtp_password": {"file": str(self.secret)}},
        }

    def test_private_file_with_spaces(self):
        checker.validate(self.config)

    def test_missing_file(self):
        self.secret.unlink()
        with self.assertRaisesRegex(ValueError, "missing"):
            checker.validate(self.config)

    def test_empty_file_and_directory(self):
        self.secret.write_text("")
        with self.assertRaisesRegex(ValueError, "non-empty regular"):
            checker.validate(self.config)
        self.secret.unlink()
        self.secret.mkdir()
        with self.assertRaisesRegex(ValueError, "non-empty regular"):
            checker.validate(self.config)

    def test_host_cannot_read(self):
        with patch.object(checker.os, "access", return_value=False):
            with self.assertRaisesRegex(ValueError, "current user"):
                checker.validate(self.config)

    def test_backend_cannot_read(self):
        self.config["services"]["backend"]["user"] = "123456:123456"
        with self.assertRaisesRegex(ValueError, "backend UID:GID"):
            checker.validate(self.config)

    def test_unconfigured_and_no_override(self):
        self.config["secrets"]["zoho_smtp_password"]["file"] = "/dev/null"
        checker.validate(self.config)
        checker.validate({"services": {"backend": {}}})

    def test_cli_does_not_echo_compose_secrets_on_failure(self):
        for body in (
            "print('synthetic-private-value'); raise SystemExit(1)",
            "print('synthetic-private-value')",
        ):
            fake = self.base / "bad-compose.py"
            fake.write_text(body)
            result = subprocess.run(
                [sys.executable, str(ROOT / "scripts/check-local-secrets.py"), sys.executable, str(fake)],
                capture_output=True, text=True,
            )
            self.assertEqual(result.returncode, 1)
            self.assertNotIn("synthetic-private-value", result.stdout + result.stderr)
            self.assertNotIn("Traceback", result.stderr)

    def test_make_stops_before_down_and_preserves_success_sequence(self):
        config_file = self.base / "compose.json"
        calls = self.base / "calls"
        fake = self.base / "compose.py"
        fake.write_text(
            "import pathlib,sys\n"
            f"config=pathlib.Path({str(config_file)!r})\n"
            f"calls=pathlib.Path({str(calls)!r})\n"
            "if sys.argv[1:] == ['config','--format','json']: print(config.read_text())\n"
            "else:\n"
            " with calls.open('a') as log: log.write(' '.join(sys.argv[1:])+'\\n')\n"
        )
        env = self.base / "local.env"
        env.write_text(f"STORAGE_PATH={self.base}/storage\nLOCAL_UID={os.getuid()}\nLOCAL_GID={os.getgid()}\n")
        command = ["make", "--no-print-directory", f"ENV_FILE={env}",
                   f"COMPOSE={sys.executable} {fake}"]
        config_file.write_text(json.dumps(self.config))
        for target in ("up", "restart"):
            result = subprocess.run(command + [target], cwd=ROOT, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(calls.read_text().splitlines(), ["down --remove-orphans", "build", "up -d"])
            calls.unlink()
        self.secret.unlink()
        for target in ("up", "restart", "run"):
            result = subprocess.run(command + [target], cwd=ROOT, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("secret file is missing", result.stderr)
            self.assertFalse(calls.exists())
            self.assertNotIn("synthetic-test-value", result.stdout + result.stderr)
        for target in ("down", "log"):
            result = subprocess.run(command + [target], cwd=ROOT, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
