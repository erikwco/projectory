package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExecutablePath preserves the installation's stable entry point. In particular,
// Linux's os.Executable resolves Homebrew's bin symlink into a versioned keg.
func ExecutablePath(invocation, executable string) string {
	current, err := os.Stat(executable)
	if err != nil {
		return executable
	}
	sameExecutable := func(candidate string) bool {
		info, err := os.Stat(candidate)
		return err == nil && os.SameFile(current, info)
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		marker := string(filepath.Separator) + "Cellar" + string(filepath.Separator) + "projectory" + string(filepath.Separator)
		if index := strings.LastIndex(resolved, marker); index >= 0 {
			stable := filepath.Join(resolved[:index], "opt", "projectory", "bin", "projectory")
			if sameExecutable(stable) {
				return stable
			}
		}
	}
	if candidate, err := exec.LookPath(invocation); err == nil {
		if absolute, err := filepath.Abs(candidate); err == nil && sameExecutable(absolute) {
			return absolute
		}
	}
	return executable
}
