package shell

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecutablePathFollowsHomebrewUpgrade(t *testing.T) {
	prefix := t.TempDir()
	prefix, err := filepath.EvalSymlinks(prefix)
	if err != nil {
		t.Fatal(err)
	}
	keg := filepath.Join(prefix, "Cellar", "projectory", "1.0.0")
	writeBinary(t, filepath.Join(keg, "bin", "projectory"), "old")
	opt := filepath.Join(prefix, "opt", "projectory")
	if err := os.MkdirAll(filepath.Dir(opt), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(keg, opt); err != nil {
		t.Fatal(err)
	}
	stable := ExecutablePath("projectory", filepath.Join(keg, "bin", "projectory"))
	if stable != filepath.Join(opt, "bin", "projectory") {
		t.Fatalf("versioned executable retained: %s", stable)
	}
	nextKeg := filepath.Join(prefix, "Cellar", "projectory", "1.1.0")
	writeBinary(t, filepath.Join(nextKeg, "bin", "projectory"), "new")
	if err := os.Remove(opt); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(nextKeg, opt); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(keg); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(stable)
	if err != nil || string(contents) != "new" {
		t.Fatalf("stable entry point failed after upgrade: %q, %v", contents, err)
	}
}

func TestExecutablePathPreservesInvokedSymlink(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "projectory-1.0.0")
	writeBinary(t, binary, "binary")
	link := filepath.Join(dir, "projectory")
	if err := os.Symlink(binary, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if got := ExecutablePath("projectory", binary); got != link {
		t.Fatalf("got %s, want %s", got, link)
	}
	other := filepath.Join(dir, "other")
	writeBinary(t, other, "different")
	if got := ExecutablePath(other, binary); got != binary {
		t.Fatalf("accepted unrelated executable: %s", got)
	}
}

func writeBinary(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0755); err != nil {
		t.Fatal(err)
	}
}
