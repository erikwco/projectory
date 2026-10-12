package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/erikwco/projectory/internal/catalog"
	"github.com/erikwco/projectory/internal/store"
)

func gitTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func TestWorktreesRefreshCopiesAndIgnoredLocations(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is not installed")
	}
	ctx := context.Background()
	root := t.TempDir()
	main := filepath.Join(root, "main")
	first := filepath.Join(root, "tool-a", "worktree")
	second := filepath.Join(root, "tool-b", "worktree")
	if err := os.MkdirAll(main, 0700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, main, "init", "-q", "-b", "main")
	gitTest(t, main, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "initial")
	gitTest(t, main, "remote", "add", "origin", "https://example.invalid/shop.git")
	gitTest(t, main, "worktree", "add", "-q", "-b", "feature/a", first)
	s, err := store.Open(filepath.Join(root, "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := &App{Store: s}
	r, err := a.Add(ctx, AddInput{Path: first, Name: "Shop", Discover: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Added != 2 {
		t.Fatalf("expected main and linked, got %+v", r)
	}
	r, err = a.Add(ctx, AddInput{Path: first, Name: "Ignored rename", Discover: true})
	if err != nil || r.Added != 0 {
		t.Fatalf("add not idempotent: %+v %v", r, err)
	}
	gitTest(t, main, "worktree", "add", "-q", "-b", "feature/b", second)
	refresh, err := a.Refresh(ctx)
	if err != nil || refresh.Added != 1 {
		t.Fatalf("refresh didn't discover worktree: %+v %v", refresh, err)
	}
	l, err := s.Resolve(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Remove(ctx, l.ID); err != nil {
		t.Fatal(err)
	}
	refresh, err = a.Refresh(ctx)
	if err != nil || refresh.Added != 0 {
		t.Fatalf("removed worktree came back: %+v %v", refresh, err)
	}
	if _, err = os.Stat(second); err != nil {
		t.Fatal("worktree was deleted:", err)
	}
	r, err = a.Add(ctx, AddInput{Path: second, Discover: true})
	if err != nil || r.Added != 1 {
		t.Fatalf("explicit re-add failed: %+v %v", r, err)
	}
	copyPath := filepath.Join(root, "copy")
	gitTest(t, root, "clone", "-q", "--no-hardlinks", main, copyPath)
	gitTest(t, copyPath, "remote", "set-url", "origin", "https://example.invalid/shop.git")
	r, err = a.Add(ctx, AddInput{Path: copyPath, Name: "Independent", Discover: true})
	if err != nil {
		t.Fatal(err)
	}
	copyLocation, err := s.Resolve(ctx, string(r.Primary))
	if err != nil {
		t.Fatal(err)
	}
	mainLocation, err := s.Resolve(ctx, main)
	if err != nil {
		t.Fatal(err)
	}
	if copyLocation.Project.ID == mainLocation.Project.ID {
		t.Fatal("same remote incorrectly merged independent repositories")
	}
	manual := filepath.Join(root, "manual-copy")
	if err = os.Mkdir(manual, 0700); err != nil {
		t.Fatal(err)
	}
	r, err = a.Add(ctx, AddInput{Path: manual, Project: "Shop", Label: "copy 2025"})
	if err != nil {
		t.Fatal(err)
	}
	grouped, err := s.Resolve(ctx, string(r.Primary))
	if err != nil || grouped.Project.ID != mainLocation.Project.ID {
		t.Fatalf("explicit copy grouping failed: %+v %v", grouped, err)
	}
	if grouped.Kind != catalog.Directory {
		t.Fatalf("copy should work without Git: %s", grouped.Kind)
	}
}

func TestNestedProjectDoesNotClaimRepositoryRoot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is not installed")
	}
	ctx := context.Background()
	root := t.TempDir()
	sub := filepath.Join(root, "packages", "client")
	if err := os.MkdirAll(sub, 0700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "init", "-q", "-b", "main")
	s, err := store.Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := &App{Store: s}
	one, err := a.Add(ctx, AddInput{Path: sub, Name: "Client", Discover: true})
	if err != nil {
		t.Fatal(err)
	}
	two, err := a.Add(ctx, AddInput{Path: root, Name: "Monorepo", Discover: true})
	if err != nil {
		t.Fatal(err)
	}
	l1, err := s.Resolve(ctx, string(one.Primary))
	if err != nil {
		t.Fatal(err)
	}
	l2, err := s.Resolve(ctx, string(two.Primary))
	if err != nil {
		t.Fatal(err)
	}
	if l1.Project.ID == l2.Project.ID || l1.IsRepoRoot || !l2.IsRepoRoot {
		t.Fatalf("nested projects were mixed: %+v %+v", l1, l2)
	}
}
