#!/usr/bin/env python3
"""Capture the actual TUI with synthetic projects in a temporary catalog.

Requires VHS, ttyd, and ffmpeg. No personal catalog or shell settings are used.
Run after make build. Captures cover wide/narrow terminals and dark/light themes.
"""

import argparse
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import tempfile


def form_steps(binary, database, output, name):
    dashboard = shlex.join([str(binary), "dash", "--query", "video-tools", "--db", str(database)])
    create = shlex.join([str(binary), "add", "--current", "--db", str(database)])

    def shot(state):
        return ["Sleep 400ms", "Show", f"Screenshot {json.dumps(str(output / (state + '-' + name + '.png')))}", "Sleep 300ms"]

    if name in ("standard", "compact", "fallback"):
        return ["Hide", f"Type {json.dumps(create)}", "Enter", *shot("form"), "Ctrl+C", "Sleep 200ms", ""]

    return [
        "Hide", f"Type {json.dumps(dashboard)}", "Enter", "Sleep 700ms", 'Type "a"',
        *shot("create"),
        "Ctrl+A", "Ctrl+K", "Ctrl+S",
        *shot("create-error"),
        "Hide", "Escape", "Sleep 200ms", 'Type "e"',
        *shot("edit"),
        "Tab 4",
        *shot("edit-path"),
        "Hide", "Escape", "Sleep 200ms", 'Type "q"', "Sleep 200ms",
        f"Type {json.dumps(create)}", "Enter",
        *shot("create-current"),
        "Hide", "Escape", "Sleep 200ms",
        "",
    ]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, default=Path(__file__).resolve().parents[1] / "bin/projectory")
    parser.add_argument("--output", type=Path, default=Path(".impeccable/review"))
    parser.add_argument("--font-family", default="Menlo")
    parser.add_argument("--font-size", type=int, default=16)
    parser.add_argument("--forms", action="store_true", help="Capture registration, editing, validation, and compact form states")
    args = parser.parse_args()
    binary, output = args.binary.resolve(), args.output.resolve()
    for tool in ("vhs", "ttyd", "ffmpeg", "git"):
        if not shutil.which(tool):
            parser.error(f"Install {tool} to capture the TUI")
    if not binary.is_file():
        parser.error("Run make build first")
    output.mkdir(parents=True, exist_ok=True)

    with tempfile.TemporaryDirectory(prefix="projectory-ui-") as directory:
        root = Path(directory).resolve()
        database = root / "catalog.db"
        env = {**os.environ, "PROJECTORY_DATA_DIR": str(root / "default"), "PROJECTORY_CONFIG_DIR": str(root / "config"), "COLORTERM": "truecolor"}
        env.pop("NO_COLOR", None)

        def run(*argv):
            return subprocess.run(argv, env=env, check=True, capture_output=True, text=True)

        primary = root / "AtlasDemo"
        primary.mkdir()
        run("git", "init", "-b", "main", str(primary))
        run("git", "-C", str(primary), "-c", "user.name=Preview", "-c", "user.email=preview@example.invalid", "commit", "--allow-empty", "-m", "Preview fixture")
        run("git", "-C", str(primary), "worktree", "add", "-b", "feature/demo", str(root / "worktrees" / "demo-feature"))
        (primary / "Package.swift").write_text("// Preview fixture\n")
        run(str(binary), "add", str(primary), "--name", "AtlasDemo", "--description", "Comunidad, pagos y comunicación entre vecinos.", "--tags", "personal,swift", "--db", str(database))
        for name, description, manifest in [
            ("data-migrator", "Migración de datos entre entornos.", "go.mod"),
            ("gateway", "Servicios de integración.", "go.mod"),
            ("notificaciones", "Mensajería y avisos.", "go.mod"),
            ("nvim", "Configuración del editor.", "init.lua"),
            ("Projectory", "Tus proyectos y sus ubicaciones, a mano.", "go.mod"),
            ("tablero.example", "Tablero de oportunidades.", "package.json"),
            ("tienda.example", "Catálogo y pedidos.", "package.json"),
            ("video-tools", "Herramientas de vídeo.", "Cargo.toml"),
        ]:
            folder = root / name
            folder.mkdir()
            (folder / manifest).write_text("{}" if manifest.endswith("json") else "// Preview fixture\n")
            run(str(binary), "add", str(folder), "--name", name, "--description", description, "--db", str(database))

        profiles = [
            ("wide", 1440, 850, "#1C232E", "#DFE6EF"),
            ("narrow", 560, 500, "#1C232E", "#DFE6EF"),
            ("light", 1440, 850, "#FAFBFC", "#253244"),
        ]
        if args.forms:
            profiles += [
                ("standard", 940, 590, "#1C232E", "#DFE6EF"),
                ("compact", 360, 280, "#1C232E", "#DFE6EF"),
                ("fallback", 176, 130, "#1C232E", "#DFE6EF"),
            ]
        for name, width, height, background, foreground in profiles:
            run(str(binary), "settings", "--search-on-open=false", "--default-sort=added")
            theme = {"name": "Projectory preview", "background": background, "foreground": foreground, "cursor": foreground, "selection": "#183840"}
            command = shlex.join([str(binary), "list", "--sort", "name", "--db", str(database)])
            settings_command = shlex.join([str(binary), "settings"])
            tape = output / f"{name}.tape"
            header = [
                f"Output {json.dumps(str(output / (name + '.webm')))}",
                'Set Shell "bash"',
                f"Set FontFamily {json.dumps(args.font_family)}",
                f"Set FontSize {args.font_size}",
                f"Set Width {width}",
                f"Set Height {height}",
                "Set Padding 20",
                "Set Framerate 10",
                "Set TypingSpeed 0ms",
                f"Set Theme {json.dumps(theme)}",
            ]
            steps = form_steps(binary, database, output, name) if args.forms else [
                "Hide",
                f"Type {json.dumps(command)}",
                "Enter",
                "Sleep 1s",
                "Show",
                f"Screenshot {json.dumps(str(output / (name + '.png')))}",
                "Sleep 300ms",
                "Enter",
                "Sleep 600ms",
                f"Screenshot {json.dumps(str(output / ('chooser-' + name + '.png')))}",
                "Sleep 300ms",
                "Down",
                "Tab",
                "Sleep 300ms",
                f"Screenshot {json.dumps(str(output / ('detail-' + name + '.png')))}",
                "Sleep 300ms",
                "Escape",
                "Escape",
                "Sleep 200ms",
                "Hide",
                'Type "q"',
                "Sleep 200ms",
                f"Type {json.dumps(settings_command)}",
                "Enter",
                "Sleep 600ms",
                "Show",
                f"Screenshot {json.dumps(str(output / ('settings-off-' + name + '.png')))}",
                "Sleep 300ms",
                'Type " "',
                "Sleep 500ms",
                f"Screenshot {json.dumps(str(output / ('settings-on-' + name + '.png')))}",
                "Sleep 300ms",
                "Down",
                "Left",
                "Sleep 500ms",
                f"Screenshot {json.dumps(str(output / ('settings-sort-' + name + '.png')))}",
                "Sleep 300ms",
                "Hide",
                "Escape",
                "Sleep 200ms",
                f"Type {json.dumps(command)}",
                "Enter",
                "Sleep 600ms",
                'Type "Atlas"',
                "Sleep 300ms",
                "Show",
                f"Screenshot {json.dumps(str(output / ('search-on-open-' + name + '.png')))}",
                "Sleep 300ms",
                "Down",
                "Sleep 300ms",
                f"Screenshot {json.dumps(str(output / ('search-navigation-' + name + '.png')))}",
                "Sleep 300ms",
                'Type "Demo"',
                "Sleep 300ms",
                f"Screenshot {json.dumps(str(output / ('search-resumed-' + name + '.png')))}",
                "Sleep 300ms",
                "Hide",
                "Ctrl+C",
                "Sleep 200ms",
                f"Type {json.dumps(settings_command + ' --search-on-open=false')}",
                "Enter",
                "Sleep 300ms",
                f"Type {json.dumps('stty cols 12 rows 4; ' + command)}",
                "Enter",
                "Sleep 500ms",
                "Show",
                f"Screenshot {json.dumps(str(output / ('fallback-' + name + '.png')))}",
                "Sleep 300ms",
                'Type "q"',
                "Sleep 200ms",
                "",
            ]
            tape.write_text("\n".join(header + steps))
            preview_cwd = root / "NewProject"
            preview_cwd.mkdir(exist_ok=True)
            subprocess.run(["vhs", str(tape)], env=env, cwd=preview_cwd, check=True)
            print(f"Captured {'forms' if args.forms else 'navigation'}: {name}", flush=True)


if __name__ == "__main__":
    main()
