package shell

import (
	"os/exec"
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
