#!/usr/bin/env python3
"""Record README GIFs from the real CLI using isolated, fictional projects."""

import argparse
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import tempfile


def main():
    repo = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, default=repo / "bin/projectory")
    parser.add_argument("--output", type=Path, default=repo / "docs/media")
    parser.add_argument("--font-family", default="monospace")
    args = parser.parse_args()
    for tool in ("vhs", "ttyd", "ffmpeg", "git", "zsh"):
        if not shutil.which(tool):
            parser.error(f"Install {tool} to record demos")
    binary, output = args.binary.resolve(), args.output.resolve()
    if not binary.is_file():
        parser.error("Run make build first")
    output.mkdir(parents=True, exist_ok=True)

    # /tmp avoids publishing OS-specific paths containing a real username.
    with tempfile.TemporaryDirectory(prefix="projectory-demo-", dir="/tmp") as directory:
        root = Path(directory).resolve()
        env = {
            "PATH": os.environ["PATH"],
            "HOME": os.environ["HOME"],
            "LANG": "en_US.UTF-8",
            "TERM": "xterm-256color",
            "COLORTERM": "truecolor",
            "ZDOTDIR": str(root),
            "PROJECTORY_DATA_DIR": str(root / "data"),
            "PROJECTORY_CONFIG_DIR": str(root / "config"),
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_CONFIG_GLOBAL": os.devnull,
        }

        def run(*argv):
            result = subprocess.run(argv, env=env, cwd=root, capture_output=True, text=True)
            if result.returncode:
                raise RuntimeError(f"{argv[0]} failed:\n{result.stdout}\n{result.stderr}")
            return result

        # zsh reads only this task's startup files, not the user's aliases/plugins.
        (root / ".zshrc").write_text(
            "PROMPT='demo > '\n"
            "unsetopt BEEP\n"
            f"eval \"$({shlex.quote(str(binary))} init zsh)\"\n"
        )
        shop = root / "tienda"
        shop.mkdir()
        (shop / "package.json").write_text('{"name":"demo-shop","private":true}\n')
        run("git", "init", "-b", "main", str(shop))
        run("git", "-C", str(shop), "add", "package.json")
        run("git", "-C", str(shop), "-c", "user.name=Demo", "-c", "user.email=demo@example.invalid",
            "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "commit", "-m", "Demo fixture")
        worktree = root / "worktrees/carrito"
        run("git", "-C", str(shop), "-c", "core.hooksPath=/dev/null", "worktree", "add", "-b", "feature/carrito", str(worktree))
        run(str(binary), "add", str(shop), "--name", "Tienda", "--description", "Catálogo y pedidos de ejemplo.", "--tags", "demo,web")
        for name, description in [("API", "Servicio de ejemplo."), ("Notas", "Documentación del equipo.")]:
            folder = root / name.lower()
            folder.mkdir()
            run(str(binary), "add", str(folder), "--name", name, "--description", description)
        (root / "portfolio").mkdir()
        run(str(binary), "settings", "--search-on-open=false", "--default-sort=name")

        def type_text(value):
            return "Type " + json.dumps(value, ensure_ascii=False)

        def record(name, steps):
            theme = {"name": "Projectory", "background": "#1C232E", "foreground": "#DFE6EF", "cursor": "#DFE6EF", "selection": "#183840"}
            header = [
                f"Output {json.dumps(str(output / (name + '.gif')))}",
                'Set Shell "zsh"', f"Set FontFamily {json.dumps(args.font_family)}",
                "Set FontSize 18", "Set Width 1120", "Set Height 640", "Set Padding 20",
                "Set Framerate 10", "Set TypingSpeed 70ms", "Set WindowBar Colorful",
                f"Set Theme {json.dumps(theme)}",
                "Hide", type_text("source " + shlex.quote(str(root / ".zshrc"))), "Enter",
                type_text("clear"), "Enter", "Sleep 300ms", "Show", "Sleep 500ms",
            ]
            tape = root / (name + ".tape")
            tape.write_text("\n".join(header + steps) + "\n")
            run("vhs", str(tape))
            print(f"Recorded {output / (name + '.gif')}", flush=True)

        record("navigation", [
            type_text("pt"), "Enter", "Sleep 2s",
            type_text("/"), type_text("tienda"), "Sleep 1500ms",
            "Enter", "Sleep 500ms", "Enter", "Sleep 2s", "Down", "Sleep 1500ms", "Enter",
            "Sleep 500ms", type_text("pwd"), "Enter", "Sleep 1500ms",
            type_text("git branch --show-current"), "Enter", "Sleep 2s",
        ])
        # Verify that the recorded navigation actually reached the worktree.
        data = json.loads(run(str(binary), "list", "--json").stdout)
        locations = data if isinstance(data, list) else data["locations"]
        assert any(item["path"] == str(worktree) and item["visits"] == 1 for item in locations), "Demo did not enter the worktree"
        record("catalog", [
            type_text("cd portfolio"), "Enter", type_text("pt add --current"), "Enter", "Sleep 1500ms",
            "Ctrl+A", "Ctrl+K", type_text("Portfolio"), "Tab", type_text("Mi sitio personal de ejemplo."),
            "Tab", type_text("demo,web"), "Sleep 1500ms", "Ctrl+S", "Sleep 1s",
            type_text("pt dash"), "Enter", "Sleep 2s", type_text("q"), "Sleep 500ms",
            type_text("pt settings"), "Enter", "Sleep 1500ms",
            type_text(" "), "Sleep 1500ms", "Escape", "Sleep 1s", "Ctrl+C", "Sleep 500ms",
        ])
        data = json.loads(run(str(binary), "list", "--json").stdout)
        locations = data if isinstance(data, list) else data["locations"]
        assert any(item["project"]["name"] == "Portfolio" for item in locations), "Demo did not save the project"


if __name__ == "__main__":
    main()
