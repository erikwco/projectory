package inspect

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/erikwco/projectory/internal/catalog"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestWorktreeDiscoveryOutsideProjectDirectory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is not installed")
	}
	ctx := context.Background()
	root := t.TempDir()
	main := filepath.Join(root, "projects", "shop")
	linked := filepath.Join(root, "tool b", "payment's worktree")
	if err := os.MkdirAll(main, 0700); err != nil {
		t.Fatal(err)
	}
	runGit(t, main, "init", "-q", "-b", "main")
	runGit(t, main, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "initial")
	runGit(t, main, "worktree", "add", "-q", "-b", "feature/payments", linked)
	c, isRoot, err := Directory(ctx, linked)
	if err != nil {
		t.Fatal(err)
	}
	if !isRoot || c.Kind != catalog.Worktree || c.Branch != "feature/payments" {
		t.Fatalf("wrong linked worktree: %+v", c)
	}
	others, err := Worktrees(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	mainPath, _ := catalog.NormalizePath(main, true)
	if len(others) != 1 || others[0].Path != mainPath || others[0].Kind != catalog.Repository {
		t.Fatalf("wrong siblings: %+v", others)
	}
	if err = os.WriteFile(filepath.Join(linked, "new file.txt"), []byte("change"), 0600); err != nil {
		t.Fatal(err)
	}
	status := WorkingStatus(ctx, linked)
	if status.Err != nil || status.Branch != "feature/payments" || status.Changed != 1 {
		t.Fatalf("wrong status: %+v", status)
	}
	runGit(t, linked, "checkout", "--detach", "-q")
	c, _, err = Directory(ctx, linked)
	if err != nil || c.Branch != "" || c.Head == "" {
		t.Fatalf("detached state: %+v %v", c, err)
	}
}

func TestPorcelainSupportsNewlinesAndBareRepos(t *testing.T) {
	got, err := ParseWorktrees("worktree /tmp/a\nline\x00HEAD abc\x00branch refs/heads/feature/x\x00\x00worktree /tmp/bare\x00bare\x00\x00")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Path != "/tmp/a\nline" || got[0].Branch != "feature/x" || !got[1].Bare {
		t.Fatalf("unexpected records: %+v", got)
	}
}

func TestDetectMixedTechnologiesWithoutExecutingManifests(t *testing.T) {
	path := t.TempDir()
	for name, data := range map[string]string{"go.mod": "module demo", "Cargo.toml": "[package]", "package.json": `{"dependencies":{"@remix-run/react":"*","react":"*"},"devDependencies":{"typescript":"*"},"scripts":{"postinstall":"exit 1"}}`} {
		if err := os.WriteFile(filepath.Join(path, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got := Technologies(path)
	for _, want := range []string{"Go", "Rust", "Node.js", "Remix", "React", "TypeScript"} {
		if !slices.Contains(got, want) {
			t.Fatalf("missing %s in %v", want, got)
		}
	}
}

func TestGitEnvironmentDoesNotRedirectInspection(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is not installed")
	}
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "correct")
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "wrong"))
	c, root, err := Directory(context.Background(), dir)
	if err != nil || !root || c.Branch != "correct" {
		t.Fatalf("external Git environment leaked: %+v %v", c, err)
	}
}

func TestCancelledInspection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := Directory(ctx, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
