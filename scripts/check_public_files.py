#!/usr/bin/env python3
"""Reject local catalogs, credentials, binaries and review captures in Git."""

from pathlib import PurePosixPath
import subprocess
import sys


def private_file(name):
    name = name.casefold()
    path = PurePosixPath(name)
    if path.parts[0] in {"bin", "dist"} or name.startswith((".impeccable/review/", ".impeccable/local-notes/")):
        return True
    if path.name.startswith(".env") and path.name not in {".env.example", ".env.sample"}:
        return True
    if path.name in {".ds_store", "id_rsa", "id_ed25519", "id_ecdsa", "id_dsa", ".netrc", "credentials.json"}:
        return True
    if "__pycache__" in path.parts or path.suffix in {".pyc", ".pyo", ".pem", ".key", ".p12", ".pfx", ".kdbx"}:
        return True
    return any(path.name.endswith(suffix) or (suffix + "-") in path.name for suffix in (".db", ".sqlite", ".sqlite3"))


files = subprocess.check_output(["git", "ls-files", "-z"]).decode().split("\0")
blocked = [name for name in files if name and private_file(name)]
if blocked:
    print("Remove private/local files from Git (contents withheld):", file=sys.stderr)
    for name in blocked:
        print(f"  {name}", file=sys.stderr)
    sys.exit(1)
print("No private local file types tracked.")
