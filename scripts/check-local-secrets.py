#!/usr/bin/env python3
"""Check the resolved local Zoho mount without reading or printing credentials."""

import json
import os
from pathlib import Path
import stat
import subprocess
import sys


def validate(config):
    backend = config.get("services", {}).get("backend", {})
    mounts = backend.get("secrets", [])
    if not any(
        (mount if isinstance(mount, str) else mount.get("source"))
        == "zoho_smtp_password"
        for mount in mounts
    ):
        return
    source = config.get("secrets", {}).get("zoho_smtp_password", {}).get("file")
    if not source:
        raise ValueError("Zoho SMTP secret has no file source.")
    # Preserve the existing explicitly unconfigured local SMTP behavior.
    if source == "/dev/null":
        return
    path = Path(source)
    try:
        info = path.stat()
    except OSError:
        raise ValueError("Zoho SMTP secret file is missing or inaccessible.") from None
    if not stat.S_ISREG(info.st_mode) or info.st_size == 0:
        raise ValueError("Zoho SMTP secret must be a non-empty regular file.")
    if not os.access(path, os.R_OK):
        raise ValueError("Zoho SMTP secret file is not readable by the current user.")
    # Local Compose bind mounts preserve host ownership; do not chmod a secret
    # broadly just to make a different container UID able to read it.
    user = str(backend.get("user", ""))
    if user:
        parts = user.split(":")
        if len(parts) != 2 or not all(part.isdecimal() for part in parts):
            raise ValueError("Cannot verify Zoho secret permissions: backend user must be UID:GID.")
        uid, gid = map(int, parts)
        read_bit = stat.S_IROTH
        if uid == info.st_uid:
            read_bit = stat.S_IRUSR
        elif gid == info.st_gid:
            read_bit = stat.S_IRGRP
        if uid != 0 and not info.st_mode & read_bit:
            raise ValueError("Zoho SMTP secret file is not readable by the backend UID:GID.")


def main():
    try:
        # Compose resolves env-file precedence, interpolation and relative paths.
        # Its output may contain other service secrets: capture it, never echo it.
        result = subprocess.run(
            sys.argv[1:] + ["config", "--format", "json"],
            capture_output=True, text=True, check=False,
        )
        if result.returncode:
            raise ValueError("Compose configuration could not be resolved; check local env files.")
        validate(json.loads(result.stdout))
    except (OSError, ValueError, TypeError) as error:
        # JSON decoder errors may contain input fragments; never print them.
        message = "Invalid Compose JSON output." if isinstance(error, json.JSONDecodeError) else str(error)
        print("Local startup stopped: " + message, file=sys.stderr)
        print(
            "Set ZOHO_SMTP_PASSWORD_FILE to a persistent private file outside the repository "
            "(mode 0600, owned by LOCAL_UID). See docs/operations/developer-bootstrap.md. "
            "No stack stop/build/start was requested by this preflight.",
            file=sys.stderr,
        )
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
