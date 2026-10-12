#!/usr/bin/env python3
"""Validate GoReleaser output against the Projectory release contract.

Release pipeline file. Checks the exact archive and checksums.txt names, SHA-256
hashes, archive members (names, types, permissions, owners, contents), the
binary format and architecture for every target, and, unless --skip-buildinfo,
the Go build info (target, CGO_ENABLED=0, -trimpath) and that every linked
module appears in THIRD_PARTY_NOTICES.md. With --native it also extracts the
archive for the current OS/architecture and runs `projectory --version`;
--native=GOOS/GOARCH additionally fails when the host is a different target.
Uses only Python's standard library (plus `go` for build info).
"""

import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import sys
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
TARGETS = [("darwin", "amd64"), ("darwin", "arm64"), ("linux", "amd64"), ("linux", "arm64")]
VERSION = re.compile(r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-rc\.[1-9]\d*)?$")
CHECKSUM_LINE = re.compile(r"^([0-9a-f]{64})  (\S+)$")
GORELEASER_METADATA = {"artifacts.json", "config.yaml", "metadata.json"}
MEMBER_MODES = {"projectory": 0o755, "LICENSE": 0o644, "README.md": 0o644, "THIRD_PARTY_NOTICES.md": 0o644}
MAIN_PATH = "github.com/erikwco/projectory/cmd/projectory"
# ELF e_machine and Mach-O cputype values per GOARCH.
ELF_MACHINE = {"amd64": 62, "arm64": 183}
MACHO_CPU = {"amd64": 0x01000007, "arm64": 0x0100000C}


class Checker:
    def __init__(self):
        self.errors = []

    def fail(self, message):
        self.errors.append(message)

    def expect(self, condition, message):
        if not condition:
            self.fail(message)
        return condition


def archive_name(version, goos, goarch):
    return f"projectory_{version}_{goos}_{goarch}.tar.gz"


def host_target():
    goos = {"Darwin": "darwin", "Linux": "linux"}.get(platform.system())
    goarch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine().lower())
    if (goos, goarch) not in TARGETS:
        raise SystemExit(f"--native: unsupported host {platform.system()}/{platform.machine()}")
    return goos, goarch


def check_dist_files(c, dist, expected):
    files = {p.name for p in dist.iterdir() if p.is_file() or p.is_symlink()}
    release_files = files - GORELEASER_METADATA
    for name in sorted(release_files - expected):
        c.fail(f"unexpected release file in {dist}: {name}")
    for name in sorted(expected - release_files):
        c.fail(f"missing release file: {name}")
    for name in sorted(expected & files):
        c.expect(not (dist / name).is_symlink(), f"{name}: must be a regular file, not a symlink")


def check_metadata(c, dist, version):
    metadata = dist / "metadata.json"
    if metadata.is_file():
        found = json.loads(metadata.read_text()).get("version")
        c.expect(found == version, f"metadata.json version {found!r} != {version!r}")


def check_checksums(c, dist, archives):
    path = dist / "checksums.txt"
    if not path.is_file():
        return
    text = path.read_text()
    c.expect(text.endswith("\n"), "checksums.txt: missing trailing newline")
    lines = text.splitlines()
    names = []
    for number, line in enumerate(lines, 1):
        match = CHECKSUM_LINE.match(line)
        if not c.expect(match, f"checksums.txt:{number}: malformed line {line!r}"):
            continue
        digest, name = match.groups()
        names.append(name)
        if (dist / name).is_file():
            actual = hashlib.sha256((dist / name).read_bytes()).hexdigest()
            c.expect(actual == digest, f"checksums.txt: {name} sha256 {actual} != listed {digest}")
    c.expect(names == sorted(archives), f"checksums.txt lists {names}, expected exactly {sorted(archives)}")


def check_binary_format(c, label, data, goos, goarch):
    if goos == "linux":
        ok = data[:4] == b"\x7fELF" and data[4] == 2 and data[5] == 1
        ok = ok and int.from_bytes(data[18:20], "little") == ELF_MACHINE[goarch]
        c.expect(ok, f"{label}: not a 64-bit little-endian ELF for {goarch}")
    else:
        ok = data[:4] == b"\xcf\xfa\xed\xfe"
        ok = ok and int.from_bytes(data[4:8], "little") == MACHO_CPU[goarch]
        c.expect(ok, f"{label}: not a 64-bit Mach-O for {goarch}")


def notices_modules(text):
    return set(re.findall(r"^\| ([^|]+?) \| (\S+) \| .+ \|$", text, re.MULTILINE))


def check_buildinfo(c, label, binary, goos, goarch, notices):
    try:
        info = subprocess.run(["go", "version", "-m", str(binary)], check=True, capture_output=True, text=True).stdout
    except (OSError, subprocess.CalledProcessError) as error:
        c.fail(f"{label}: go version -m failed: {error}")
        return
    fields = [line.strip().split("\t") for line in info.splitlines()[1:]]
    build = {f[1].split("=", 1)[0]: f[1].split("=", 1)[1] for f in fields if f[0] == "build" and "=" in f[1]}
    paths = [f[1] for f in fields if f[0] == "path"]
    expected = {"GOOS": goos, "GOARCH": goarch, "CGO_ENABLED": "0", "-trimpath": "true"}
    for key, value in expected.items():
        c.expect(build.get(key) == value, f"{label}: build {key}={build.get(key)!r}, expected {value!r}")
    c.expect(paths == [MAIN_PATH], f"{label}: main package {paths}, expected {MAIN_PATH}")

    listed = notices_modules(notices)
    goversion = info.splitlines()[0].rsplit(" ", 1)[-1]
    c.expect(("Go standard library", goversion) in listed,
             f"{label}: THIRD_PARTY_NOTICES.md lacks Go standard library {goversion}")
    for i, field in enumerate(fields):
        if field[0] != "dep":
            continue
        module = (field[1], field[2])
        if i + 1 < len(fields) and fields[i + 1][0] == "=>":
            module = (fields[i + 1][1], fields[i + 1][2])
        c.expect(module in listed, f"{label}: linked module {module[0]} {module[1]} missing from THIRD_PARTY_NOTICES.md")


def check_archive(c, path, version, goos, goarch, buildinfo, workdir):
    label = path.name
    try:
        tar = tarfile.open(path, "r:gz")
    except (OSError, tarfile.TarError) as error:
        c.fail(f"{label}: not a readable tar.gz ({error})")
        return None
    contents = {}
    with tar:
        members = tar.getmembers()
        names = [m.name for m in members]
        c.expect(len(names) == len(set(names)), f"{label}: duplicate members {sorted(names)}")
        c.expect(set(names) == set(MEMBER_MODES), f"{label}: members {sorted(names)}, expected {sorted(MEMBER_MODES)}")
        for member in members:
            where = f"{label}:{member.name}"
            if not c.expect(member.isreg(), f"{where}: must be a regular file"):
                continue
            expected_mode = MEMBER_MODES.get(member.name)
            c.expect(member.mode == expected_mode, f"{where}: mode {oct(member.mode)}, expected {oct(expected_mode or 0)}")
            c.expect((member.uid, member.gid) == (0, 0), f"{where}: owner {member.uid}:{member.gid}, expected 0:0")
            contents[member.name] = tar.extractfile(member).read()

    for name in ("LICENSE", "README.md"):
        if name in contents:
            c.expect(contents[name] == (ROOT / name).read_bytes(), f"{label}:{name}: differs from repository {name}")
    notices = contents.get("THIRD_PARTY_NOTICES.md", b"").decode("utf-8", "replace")
    c.expect(notices.startswith("# Third-party notices\n"), f"{label}:THIRD_PARTY_NOTICES.md: missing or not generated")

    binary = contents.get("projectory")
    if binary is None:
        return contents
    check_binary_format(c, f"{label}:projectory", binary, goos, goarch)
    c.expect(version.encode() in binary, f"{label}:projectory: version string {version} not embedded")
    if buildinfo:
        target = workdir / f"{goos}_{goarch}" / "projectory"
        target.parent.mkdir()
        target.write_bytes(binary)
        check_buildinfo(c, f"{label}:projectory", target, goos, goarch, notices)
    return contents


def run_native(c, binary, version, workdir):
    target = workdir / "native" / "projectory"
    target.parent.mkdir()
    target.write_bytes(binary)
    target.chmod(0o755)
    env = {"PATH": os.environ.get("PATH", "/usr/bin:/bin"), "HOME": str(workdir), "LC_ALL": "C"}
    try:
        result = subprocess.run([str(target), "--version"], capture_output=True, text=True, timeout=30, env=env, cwd=workdir)
    except (OSError, subprocess.TimeoutExpired) as error:
        c.fail(f"native: cannot execute projectory --version: {error}")
        return
    expected = f"projectory version {version}\n"
    c.expect(result.returncode == 0 and result.stdout == expected,
             f"native: projectory --version exited {result.returncode} with {result.stdout!r}, expected {expected!r}")


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--dist", type=Path, default=ROOT / "dist", help="directory with archives and checksums.txt")
    parser.add_argument("--version", required=True, help="release version without the leading v, e.g. 1.0.0-rc.1")
    parser.add_argument("--native", nargs="?", const="host", metavar="GOOS/GOARCH",
                        help="also run the binary for the current OS/architecture; with a value, require that host")
    parser.add_argument("--skip-buildinfo", action="store_true", help="skip `go version -m` checks (no Go toolchain)")
    args = parser.parse_args()

    if not VERSION.match(args.version):
        raise SystemExit(f"version {args.version!r} does not match MAJOR.MINOR.PATCH[-rc.N] (no leading v)")
    if not args.dist.is_dir():
        raise SystemExit(f"{args.dist}: not a directory")

    c = Checker()
    archives = {archive_name(args.version, goos, goarch): (goos, goarch) for goos, goarch in TARGETS}
    check_dist_files(c, args.dist, set(archives) | {"checksums.txt"})
    check_metadata(c, args.dist, args.version)
    check_checksums(c, args.dist, archives)

    native = host_target() if args.native else None
    if args.native not in (None, "host") and "/".join(native) != args.native:
        raise SystemExit(f"--native: host is {'/'.join(native)}, expected {args.native}")
    notices = set()
    with tempfile.TemporaryDirectory(prefix="projectory-release-check-") as tmp:
        workdir = Path(tmp)
        for name, (goos, goarch) in sorted(archives.items()):
            if not (args.dist / name).is_file():
                continue
            contents = check_archive(c, args.dist / name, args.version, goos, goarch, not args.skip_buildinfo, workdir)
            if contents is None:
                continue
            notices.add(contents.get("THIRD_PARTY_NOTICES.md"))
            if (goos, goarch) == native and "projectory" in contents:
                run_native(c, contents["projectory"], args.version, workdir)
    c.expect(len(notices) <= 1, "THIRD_PARTY_NOTICES.md differs between archives")

    if c.errors:
        print(f"Release artifacts violate the contract ({len(c.errors)} problems):", file=sys.stderr)
        for error in c.errors:
            print(f"  - {error}", file=sys.stderr)
        return 1
    checks = ["names", "checksums", "members", "permissions", "binary formats"]
    if not args.skip_buildinfo:
        checks.append("build info and notices coverage")
    if native:
        checks.append(f"native {native[0]}/{native[1]} --version")
    print(f"Release artifacts for {args.version} OK: {', '.join(checks)}.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
