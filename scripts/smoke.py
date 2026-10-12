#!/usr/bin/env python3
"""Exercise the built CLI, parent shells, and TUI through a real pseudo-terminal.

Uses only Python's standard library. All catalogs and projects live in a temporary
directory; the user's catalog and shell configuration are never used.
"""

import argparse
import errno
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import shutil
import signal
import struct
import subprocess
import tempfile
import termios
import time


class Terminal:
    def __init__(self, argv, env, cwd, width=80, height=24):
        self.master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))

        def controlling_terminal():
            os.setsid()
            fcntl.ioctl(0, termios.TIOCSCTTY, 0)

        self.process = subprocess.Popen(
            argv, stdin=slave, stdout=slave, stderr=slave, cwd=cwd,
            env={**env, "TERM": "xterm-256color"}, preexec_fn=controlling_terminal,
        )
        os.close(slave)
        self.output = bytearray()

    def read(self, seconds=0.2):
        deadline = time.monotonic() + seconds
        while time.monotonic() < deadline:
            ready, _, _ = select.select([self.master], [], [], min(0.05, max(0, deadline - time.monotonic())))
            if not ready:
                continue
            try:
                chunk = os.read(self.master, 65536)
            except OSError as error:
                if error.errno == errno.EIO:
                    break
                raise
            if not chunk:
                break
            self.output.extend(chunk)
            # Answer the background query as a dark terminal. Other optional
            # capabilities remain unsupported, as on a basic terminal.
            if b"\x1b]11;?" in chunk:
                os.write(self.master, b"\x1b]11;rgb:1111/1111/1111\x1b\\")
        return self.output.decode("utf-8", errors="replace")

    def expect(self, text, timeout=8, after=0):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            self.read()
            if text in self.output[after:].decode("utf-8", errors="replace"):
                return
            if self.process.poll() is not None:
                break
        raise AssertionError(f"Terminal did not show {text!r}:\n{self.output.decode(errors='replace')}")

    def send(self, text):
        os.write(self.master, text.encode() if isinstance(text, str) else text)

    def resize(self, width, height):
        fcntl.ioctl(self.master, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
        os.killpg(self.process.pid, signal.SIGWINCH)
        self.read()

    def finish(self, timeout=8):
        deadline = time.monotonic() + timeout
        while self.process.poll() is None and time.monotonic() < deadline:
            self.read()
        if self.process.poll() is None:
            raise AssertionError("Terminal process did not exit")
        self.read()
        assert self.process.returncode == 0, self.output.decode(errors="replace")
        return self.output.decode(errors="replace")

    def close(self):
        if self.process.poll() is None:
            os.killpg(self.process.pid, signal.SIGTERM)
            self.process.wait(timeout=5)
        os.close(self.master)

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, default=Path(__file__).resolve().parents[1] / "bin/projectory")
    args = parser.parse_args()
    binary = args.binary.resolve()
    assert binary.is_file(), f"Build first: go build -o {binary} ./cmd/projectory"

    with tempfile.TemporaryDirectory(prefix="projectory-smoke-") as directory:
        root = Path(directory).resolve()
        database = root / "custom catalog.db"
        env = {**os.environ, "PROJECTORY_DATA_DIR": str(root / "unused-default"), "PROJECTORY_CONFIG_DIR": str(root / "config"), "PROJECTORY_BIN": str(binary), "PROJECTORY_TEST_DB": str(database)}

        def cli(*argv, ok=True):
            result = subprocess.run([str(binary), *argv, "--db", str(database)], cwd=root, env=env, capture_output=True, text=True)
            if ok:
                assert result.returncode == 0, f"{argv}: {result.stderr}"
            else:
                assert result.returncode != 0, f"{argv}: should fail"
            return result

        def locations():
            return json.loads(cli("list", "--json").stdout)

        target = root / "work's $project; [one]\nwith newline"
        target.mkdir()
        (target / "go.mod").write_text("module example.invalid/demo\n")
        cli("add", str(target), "--name", "Pickme", "--tags", "personal, tools")
        cli("add", str(target), "--name", "Should not rename")
        records = locations()
        assert len(records) == 1 and records[0]["project"]["name"] == "Pickme"
        assert records[0]["technologies"] == ["Go"]
        identity = records[0]["id"]
        env.update(PROJECTORY_TARGET=str(target), PROJECTORY_LOCATION_ID=identity)
        print("PASS CLI persistence, technology detection, and duplicate registration")

        shells = [name for name in ("zsh", "bash") if shutil.which(name)]
        assert shells, "zsh or bash is required for shell integration checks"
        for shell in shells:
            code = r'''
eval "$("$PROJECTORY_BIN" init "$PROJECTORY_SHELL")"
pt update --help > "$PROJECTORY_UPDATE_HELP" || exit 18
pt go "$PROJECTORY_LOCATION_ID" --db "$PROJECTORY_TEST_DB" || exit 10
[ "$PWD" = "$PROJECTORY_TARGET" ] || exit 11
pt go missing-location --db "$PROJECTORY_TEST_DB" >/dev/null 2>&1 && exit 12
[ "$PWD" = "$PROJECTORY_TARGET" ] || exit 13
pt ~ || exit 14
[ "$PWD" = "$HOME" ] || exit 15
builtin cd -- "$PROJECTORY_TARGET"
pt '~' || exit 16
[ "$PWD" = "$HOME" ] || exit 17
'''
            update_help = root / f"update-help-{shell}.txt"
            result = subprocess.run([shell, "-f" if shell == "zsh" else "--noprofile", "-c", code], cwd=root, env={**env, "PROJECTORY_SHELL": shell, "PROJECTORY_UPDATE_HELP": str(update_help)}, capture_output=True, text=True)
            assert result.returncode == 0, f"{shell}: {result.stderr}"
            assert "--check" in update_help.read_text(), f"{shell}: pt did not forward the update command"
        assert locations()[0]["visits"] == len(shells)
        assert not (root / "unused-default" / "catalog.db").exists(), "visit was recorded in the wrong database"
        print(f"PASS actual cd, pt ~, and visit accounting in {', '.join(shells)}, including unusual paths")

        for shell in shells:
            prefix = root / f"brew-{shell}"
            old_keg = prefix / "Cellar/projectory/old"
            new_keg = prefix / "Cellar/projectory/new"
            for keg in (old_keg, new_keg):
                (keg / "bin").mkdir(parents=True)
                shutil.copy2(binary, keg / "bin/projectory")
            (prefix / "opt").mkdir()
            (prefix / "opt/projectory").symlink_to(old_keg, target_is_directory=True)
            (prefix / "bin").mkdir()
            (prefix / "bin/projectory").symlink_to(old_keg / "bin/projectory")
            code = r'''
set -e
eval "$("$PROJECTORY_BIN" init "$PROJECTORY_SHELL")"
rm "$PROJECTORY_BREW_PREFIX/opt/projectory"
ln -s "$PROJECTORY_BREW_PREFIX/Cellar/projectory/new" "$PROJECTORY_BREW_PREFIX/opt/projectory"
rm "$PROJECTORY_BREW_PREFIX/Cellar/projectory/old/bin/projectory"
pt --version
'''
            result = subprocess.run([shell, "-f" if shell == "zsh" else "--noprofile", "-c", code], cwd=root, env={**env, "PROJECTORY_SHELL": shell, "PROJECTORY_BIN": str(prefix / "bin/projectory"), "PROJECTORY_BREW_PREFIX": str(prefix)}, capture_output=True, text=True)
            assert result.returncode == 0 and "projectory version" in result.stdout, f"{shell}: function broke after Homebrew cleanup: {result.stderr}"
        print("PASS open shell function survives a replaced Homebrew keg and removal of the old binary")

        # Noninteractive output and cancellation must never select a directory.
        chosen = root / "selection"
        cli("list", "--plain", "--selection-file", str(chosen))
        assert not chosen.exists()
        with Terminal([str(binary), "list", "--db", str(database), "--selection-file", str(chosen)], env, root) as terminal:
            terminal.expect("PROJECTORY")
            terminal.send("q")
            terminal.finish()
        assert not chosen.exists()
        print("PASS cancellation and plain output leave navigation untouched")

        shell = shells[0]
        code = r'''
eval "$("$PROJECTORY_BIN" init "$PROJECTORY_SHELL")"
pt list --db "$PROJECTORY_TEST_DB" || exit 20
[ "$PWD" = "$PROJECTORY_TARGET" ] || exit 21
printf '\n__PROJECTORY_NAVIGATION_OK__\n'
'''
        with Terminal([shell, "-f" if shell == "zsh" else "--noprofile", "-c", code], {**env, "PROJECTORY_SHELL": shell}, root, 120, 30) as terminal:
            terminal.expect("PROJECTORY")
            terminal.resize(42, 14)
            terminal.resize(160, 45)
            terminal.send("/Pickmex")
            terminal.expect("No hay proyectos que coincidan.")
            changed_at = len(terminal.output)
            terminal.send("\x7f")
            terminal.expect("> Pickme", after=changed_at)
            assert terminal.process.poll() is None, "typing navigated prematurely"
            assert locations()[0]["visits"] == len(shells), "typing counted a visit"
            terminal.send("\r")
            terminal.read(0.5)
            assert terminal.process.poll() is None, "focus Enter navigated prematurely"
            assert locations()[0]["visits"] == len(shells), "focus change counted a visit"
            terminal.send("\r")
            output = terminal.finish()
            assert "__PROJECTORY_NAVIGATION_OK__" in output
        assert locations()[0]["visits"] == len(shells) + 1
        print("PASS live TUI search, backspace, focus, resize, selection, parent-shell cd, and visit end to end")

        # Preferences are global to the user, independent of the selected catalog.
        assert "desactivado" in cli("settings").stdout
        with Terminal([str(binary), "list", "--db", str(database)], env, root) as terminal:
            terminal.expect("PROJECTORY")
            terminal.send("\x1bOQ")  # F2
            terminal.expect("Buscar al abrir")
            terminal.send(" ")
            terminal.expect("Guardado")
            terminal.send("\x1b")
            terminal.read()
            terminal.send("q")
            terminal.finish()
        config_path = root / "config" / "settings.json"
        assert json.loads(config_path.read_text())["search_on_open"] is True
        other_catalog = subprocess.run([str(binary), "settings", "--db", str(root / "unused.db")], env=env, capture_output=True, text=True, check=True)
        assert "Buscar al abrir: activado" in other_catalog.stdout
        assert not (root / "unused.db").exists(), "settings opened a catalog"
        for command in ("list", "dash"):
            with Terminal([str(binary), command, "--db", str(database), "--selection-file", str(chosen)], env, root) as terminal:
                terminal.expect("PROJECTORY")
                terminal.send("Pickmex")
                terminal.expect("No hay proyectos que coincidan.")
                changed_at = len(terminal.output)
                terminal.send("\x7f")
                terminal.expect("> Pickme", after=changed_at)
                terminal.send("\r")
                terminal.read(0.3)
                assert terminal.process.poll() is None, "search-on-open Enter navigated prematurely"
                assert not chosen.exists(), "focus Enter selected a location"
                terminal.send("\r")
                terminal.finish()
            assert str(target).encode() in chosen.read_bytes()
            chosen.unlink()
        neighbor = root / "Pickother"
        neighbor.mkdir()
        cli("add", str(neighbor))
        neighbor_id = next(record["id"] for record in locations() if record["path"] == str(neighbor))
        with Terminal([str(binary), "dash", "--sort", "name", "--db", str(database), "--selection-file", str(chosen)], env, root) as terminal:
            terminal.expect("PROJECTORY")
            terminal.send("Pick\x1b[B")
            terminal.expect("> Pickother")
            terminal.send("\x05")  # Ctrl+E opens editing from the list.
            terminal.expect("Editar ubicaci")
            terminal.send("\x1b")
            terminal.read()
            terminal.send("\x04")  # Ctrl+D keeps removal behind confirmation.
            terminal.expect("Quitar del cat")
            terminal.send("n")
            terminal.read()
            terminal.send("\x01")  # Ctrl+A opens registration from the list.
            terminal.expect("Registrar ubicaci")
            terminal.send("\x1b")
            terminal.read()
            terminal.send("q")  # A letter after navigation resumes the query.
            terminal.expect("No hay proyectos que coincidan.")
            assert terminal.process.poll() is None, "typing q exited automatic search"
            assert not chosen.exists(), "arrow movement selected a location prematurely"
            changed_at = len(terminal.output)
            terminal.send("\x7f\x1b[B")
            terminal.expect("> Pickother", after=changed_at)
            terminal.send("\r")  # Arrow already focused the list: one Enter selects.
            terminal.finish()
        assert chosen.read_bytes().split(b"\x00")[1] == str(neighbor).encode()
        chosen.unlink()
        cli("remove", neighbor_id, "--yes")
        print("PASS arrow focus and movement, resumed typing, Ctrl dashboard actions, and one-Enter selection")
        # The direct settings screen offers the same toggle and persists disabling it.
        with Terminal([str(binary), "settings"], env, root) as terminal:
            terminal.expect("Buscar al abrir")
            terminal.send("\r")
            terminal.expect("Guardado")
            terminal.send("\x1b")
            terminal.finish()
        assert json.loads(config_path.read_text())["search_on_open"] is False
        cli("settings", "--search-on-open=true")
        assert json.loads(config_path.read_text())["search_on_open"] is True
        cli("settings", "--search-on-open=false")
        assert json.loads(config_path.read_text())["search_on_open"] is False
        assert locations()[0]["visits"] == len(shells) + 1, "settings changed visit accounting"
        print("PASS F2 settings, saved toggle, automatic search in list and dash, two-step Enter, and manual-search restoration")

        with Terminal([str(binary), "settings"], env, root, 40, 14) as terminal:
            terminal.expect("Orden predeterminado")
            terminal.send("\x1b[B\x1b[D")  # Select order; cycle backward from registration to name.
            terminal.expect("Guardado")
            terminal.send("\x1b")
            terminal.finish()
        assert json.loads(config_path.read_text())["default_sort"] == "name"
        for command in ("list", "dash"):
            with Terminal([str(binary), command, "--db", str(database)], env, root) as terminal:
                terminal.expect("orden: nombre")
                terminal.send("q")
                terminal.finish()
        with Terminal([str(binary), "list", "--sort", "visited", "--db", str(database)], env, root) as terminal:
            terminal.expect("orden: última visita")
            terminal.send("q")
            terminal.finish()
        assert json.loads(config_path.read_text())["default_sort"] == "name", "explicit order replaced preference"
        cli("settings", "--default-sort=added")
        print("PASS saved default order, settings navigation, fresh list/dashboard, and explicit sort override")

        # A project has one row even when Git discovers an external worktree.
        primary = root / "GroupedProject"
        worktree = root / "external" / "grouped-feature"
        primary.mkdir()
        subprocess.run(["git", "init", "-b", "main", str(primary)], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(primary), "-c", "user.name=Projectory Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "initial"], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(primary), "worktree", "add", "-b", "feature/chooser-proof", str(worktree)], check=True, capture_output=True)
        cli("add", str(primary), "--name", "GroupedProject")
        grouped = [record for record in locations() if record["project"]["name"] == "GroupedProject"]
        assert len(grouped) == 2
        primary_id = next(record["id"] for record in grouped if record["path"] == str(primary))
        worktree_id = next(record["id"] for record in grouped if record["path"] == str(worktree))
        code = r'''
eval "$("$PROJECTORY_BIN" init "$PROJECTORY_SHELL")"
pt list --query chooser-proof --db "$PROJECTORY_TEST_DB" || exit 30
[ "$PWD" = "$PROJECTORY_TARGET" ] || exit 31
printf '\n__PROJECTORY_WORKTREE_OK__\n'
'''
        with Terminal([shell, "-f" if shell == "zsh" else "--noprofile", "-c", code], {**env, "PROJECTORY_SHELL": shell, "PROJECTORY_TARGET": str(worktree)}, root, 120, 30) as terminal:
            terminal.expect("1 de 2 proyectos")
            terminal.send("\r")
            terminal.expect("Abrir ubicaci")
            assert terminal.process.poll() is None, "project Enter navigated before choosing a location"
            assert all(record["visits"] == 0 for record in locations() if record["project"]["name"] == "GroupedProject")
            terminal.resize(40, 12)
            terminal.resize(120, 30)
            terminal.send("\t")
            terminal.expect("Detalle de la ubicaci")
            terminal.send("\x1b")
            terminal.read()
            terminal.send("\r")
            assert "__PROJECTORY_WORKTREE_OK__" in terminal.finish()
        grouped = [record for record in locations() if record["project"]["name"] == "GroupedProject"]
        assert next(record["visits"] for record in grouped if record["id"] == worktree_id) == 1
        assert next(record["visits"] for record in grouped if record["id"] == primary_id) == 0
        print("PASS project grouping, worktree chooser, detail return, resize, selected path and per-location visits")

        with Terminal([str(binary), "dash", "--query", "GroupedProject", "--db", str(database)], env, root, 100, 28) as terminal:
            terminal.expect("dashboard")
            terminal.send("d")
            terminal.expect("Quitar ubicaci")
            terminal.send("\x1b[B\r")
            terminal.expect("Quitar del cat")
            terminal.send("n")
            terminal.read()
            assert len(locations()) == 3, "cancelled grouped removal changed the catalog"
            terminal.send("\rs")
            terminal.expect("quitada del cat")
            terminal.send("q")
            terminal.finish()
        assert not any(record["id"] == worktree_id for record in locations())
        assert any(record["id"] == primary_id for record in locations())
        assert worktree.is_dir(), "removal deleted worktree files"
        cli("remove", primary_id, "--yes")
        print("PASS grouped removal targets only the chosen location and preserves its files")

        form_path = root / "FormProject"
        form_path.mkdir()
        with Terminal([str(binary), "add", "--current", "--db", str(database)], env, form_path) as terminal:
            terminal.expect("Registrar proyecto")
            terminal.send("\x01\x0b\x13")
            terminal.expect("Completa nombre del proyecto")
            assert not any(record["path"] == str(form_path) for record in locations())
            terminal.resize(12, 4)
            terminal.expect("q salir")
            terminal.send("hidden\x13")
            terminal.read()
            assert terminal.process.poll() is None, "hidden form submitted"
            terminal.resize(80, 24)
            terminal.send("FormProject")
            terminal.send("\tAdded from terminal form\tpersonal, test\x13")
            terminal.finish()
        form_record = next(record for record in locations() if record["path"] == str(form_path))
        assert form_record["project"]["description"] == "Added from terminal form"
        assert form_record["project"]["tags"] == ["personal", "test"]
        print("PASS form validation, tiny-terminal input guard, resize recovery, and registration metadata")

        with Terminal([str(binary), "dash", "--query", "FormProject", "--db", str(database)], env, root) as terminal:
            terminal.expect("dashboard")
            terminal.send("e")
            terminal.expect("Editar ubicaci")
            terminal.send("\t\t\t\t\x05")  # Path end remains visible after narrowing.
            terminal.read()
            changed_at = len(terminal.output)
            terminal.resize(24, 8)
            terminal.expect("FormProject", after=changed_at)
            terminal.expect("Ctrl+S", after=changed_at)
            terminal.expect("Esc", after=changed_at)
            terminal.resize(80, 24)
            terminal.send("\t")
            terminal.send("\x01RenamedProject\x0b\x13")
            terminal.expect("Cambios guardados")
            terminal.send("/\x01\x0bRenamedProject\r")
            terminal.read()
            terminal.send("d")
            terminal.expect("Quitar del cat")
            terminal.send("n")
            terminal.read()
            assert len(locations()) == 2, "cancelled deletion removed a record"
            terminal.send("ds")
            terminal.expect("quitada del cat")
            terminal.send("q")
            terminal.finish()
        assert len(locations()) == 1 and form_path.is_dir()
        print("PASS dashboard editing, deletion confirmation, and file preservation")

        backup = root / "backup.db"
        cli("backup", str(backup))
        cli("backup", str(backup), ok=False)
        result = subprocess.run([str(binary), "list", "--json", "--db", str(backup)], env=env, capture_output=True, text=True, check=True)
        assert json.loads(result.stdout)[0]["visits"] == len(shells) + 1
        print("PASS consistent SQLite backup, including visit history")

    print("All executable and terminal smoke checks passed.")


if __name__ == "__main__":
    main()
