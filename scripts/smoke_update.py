#!/usr/bin/env python3
"""Verify an official update using a disposable installation and user data.

This deliberately contacts GitHub. It is a release-maintainer check, not part of
make check. Pass an older published binary (or a release-candidate test build).
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--expected-version", required=True)
    parser.add_argument("--prerelease", action="store_true")
    args = parser.parse_args()
    source = args.binary.resolve(strict=True)
    shell = shutil.which("zsh") or shutil.which("bash")
    if not shell:
        parser.error("zsh or bash is required")

    with tempfile.TemporaryDirectory(prefix="projectory-update-smoke-") as directory:
        root = Path(directory)
        target = root / "bin" / "projectory"
        target.parent.mkdir()
        shutil.copy2(source, target)
        data = root / "data"
        config = root / "config"
        project = root / "example-project"
        project.mkdir()
        env = {**os.environ, "PROJECTORY_DATA_DIR": str(data), "PROJECTORY_CONFIG_DIR": str(config)}

        def run(*args):
            return subprocess.check_output([str(target), *args], env=env, text=True).strip()

        old_version = run("--version")
        assert old_version != f"projectory version {args.expected_version}", "input is already the target version"
        run("add", str(project), "--name", "Release smoke", "--tags", "test")
        run("settings", "--search-on-open", "--default-sort", "name")
        records = json.loads(run("list", "--json"))
        catalog = data / "catalog.db"
        settings = config / "settings.json"
        before_catalog, before_settings = digest(catalog), digest(settings)
        before_binary = digest(target)
        candidate_flag = ["--prerelease"] if args.prerelease else []
        print(run("update", "--check", *candidate_flag), flush=True)
        assert digest(target) == before_binary, "--check modified the executable"

        # Keep pt alive across replacement, as a user does in an open terminal.
        shell_name = Path(shell).name
        env.update(PROJECTORY_SMOKE_BINARY=str(target), PROJECTORY_SMOKE_SHELL=shell_name)
        code = '''set -e
eval "$("$PROJECTORY_SMOKE_BINARY" init "$PROJECTORY_SMOKE_SHELL")"
pt update --yes "$@"
pt --version
'''
        command = [shell, "-f" if shell_name == "zsh" else "--noprofile", "-c", code, "update-smoke", *candidate_flag]
        output = subprocess.check_output(command, env=env, text=True)
        print(output, end="", flush=True)
        assert f"projectory version {args.expected_version}" in output
        assert digest(target) != before_binary, "update did not replace the binary"
        assert digest(catalog) == before_catalog, "update changed the catalog"
        assert digest(settings) == before_settings, "update changed settings"
        assert json.loads(run("list", "--json")) == records, "catalog records changed"
        assert run("--version") == f"projectory version {args.expected_version}"
        assert not list(target.parent.glob(".projectory-update-*")), "update left temporary files"
        print("PASS official update, open shell function, catalog and settings preservation")


if __name__ == "__main__":
    main()
