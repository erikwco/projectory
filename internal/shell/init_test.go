package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomeShortcutChangesParentShellWithoutCallingBinary(t *testing.T) {
	for _, shell := range []string{"zsh", "bash"} {
		t.Run(shell, func(t *testing.T) {
			path, err := exec.LookPath(shell)
			if err != nil {
				t.Skip("shell unavailable")
			}
			init, err := Init(shell, "pt", "/nonexistent/projectory-test-binary")
			if err != nil {
				t.Fatal(err)
			}
			start := t.TempDir()
			script := init + `
set -e
builtin cd -- ` + quote(start) + `
pt ~
[ "$PWD" = "$HOME" ]
builtin cd -- ` + quote(start) + `
pt '~'
[ "$PWD" = "$HOME" ]
builtin cd -- ` + quote(start) + `
pt "$HOME"
[ "$PWD" = "$HOME" ]
builtin cd -- ` + quote(start) + `
if pt '~' unexpected 2>/dev/null; then exit 10; fi
[ "$PWD" = ` + quote(start) + ` ]
`
			if output, err := exec.Command(path, "-c", script).CombinedOutput(); err != nil {
				t.Fatalf("home shortcut failed: %v\n%s", err, output)
			}
		})
	}
}

// fakeBinary records its arguments and answers like projectory: "nav" writes a
// selection, "fail" exits with an error, anything else writes nothing.
const fakeBinary = `#!/bin/sh
printf '%s|' "$@" >> "$PT_LOG"
printf '\036' >> "$PT_LOG"
file="${1#--selection-file=}"
case "$2" in
  nav) printf 'id\000%s\000db\000' "$PT_TARGET" > "$file" ;;
  fail) exit 3 ;;
esac
`

func TestFunctionNavigatesOnlyWhenBinaryWritesALocation(t *testing.T) {
	for _, shell := range []string{"zsh", "bash"} {
		t.Run(shell, func(t *testing.T) {
			path, err := exec.LookPath(shell)
			if err != nil {
				t.Skip("shell unavailable")
			}
			dir := t.TempDir()
			binary := filepath.Join(dir, "fake projectory")
			if err := os.WriteFile(binary, []byte(fakeBinary), 0700); err != nil {
				t.Fatal(err)
			}
			init, err := Init(shell, "pt", binary)
			if err != nil {
				t.Fatal(err)
			}
			start := filepath.Join(dir, "start")
			target := filepath.Join(dir, "$(touch pwned); `touch pwned2` [x]\nline")
			temp := filepath.Join(dir, "tmp")
			for _, folder := range []string{start, target, temp} {
				if err := os.Mkdir(folder, 0700); err != nil {
					t.Fatal(err)
				}
			}
			script := init + `
builtin cd -- "$PT_START"
pt -- -dash || exit 10
[ "$PWD" = "$PT_START" ] || exit 11
pt fail
[ "$?" -eq 3 ] || exit 12
[ "$PWD" = "$PT_START" ] || exit 13
pt nav || exit 14
[ "$PWD" = "$PT_TARGET" ] || exit 15
`
			cmd := exec.Command(path, "-c", script)
			log := filepath.Join(dir, "log")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PT_LOG="+log, "PT_START="+start, "PT_TARGET="+target, "TMPDIR="+temp)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("function failed: %v\n%s", err, output)
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(calls), "\x1e")
			if len(lines) < 4 || !strings.HasPrefix(lines[0], "--selection-file=") || !strings.HasSuffix(lines[0], "|--|-dash|") {
				t.Fatalf("selection file must precede the arguments:\n%s", calls)
			}
			if !strings.HasPrefix(lines[3], "_visit|id|--path|") || !strings.HasSuffix(lines[3], "|--db|db|") {
				t.Fatalf("visit was not recorded after cd:\n%s", calls)
			}
			for _, name := range []string{"pwned", "pwned2"} {
				if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
					t.Fatal("a selected path was executed as code")
				}
			}
			if leftovers, _ := os.ReadDir(temp); len(leftovers) != 0 {
				t.Fatalf("selection files were left behind: %v", leftovers)
			}
		})
	}
}
