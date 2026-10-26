package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/erikwco/projectory/internal/catalog"
	"github.com/erikwco/projectory/internal/store"
)

func navigationApp(t *testing.T) (*App, catalog.Location) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	a := &App{Store: s}
	r, err := a.Add(context.Background(), AddInput{Path: t.TempDir(), Name: "Tienda"})
	if err != nil {
		t.Fatal(err)
	}
	l, err := s.Resolve(context.Background(), string(r.Primary))
	if err != nil {
		t.Fatal(err)
	}
	return a, l
}

func missingLocation(t *testing.T, a *App, l catalog.Location) *MissingLocation {
	t.Helper()
	if err := os.Remove(l.Path); err != nil {
		t.Fatal(err)
	}
	err := a.CheckNavigation(l)
	var missing *MissingLocation
	if !errors.As(err, &missing) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("absent folder must produce a wrapped missing-location error: %v", err)
	}
	return missing
}

func TestNavigationOnlyOffersCleanupForAbsentDirectories(t *testing.T) {
	for _, kind := range []string{"directory", "missing", "file", "ENOTDIR", "permission"} {
		t.Run(kind, func(t *testing.T) {
			a, l := navigationApp(t)
			var cause error
			switch kind {
			case "missing", "file":
				if err := os.Remove(l.Path); err != nil {
					t.Fatal(err)
				}
				if kind == "file" {
					if err := os.WriteFile(l.Path, []byte("keep"), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					cause = os.ErrNotExist
				}
			case "ENOTDIR":
				file := filepath.Join(l.Path, "file")
				if err := os.WriteFile(file, nil, 0600); err != nil {
					t.Fatal(err)
				}
				l.Path = filepath.Join(file, "child")
				cause = syscall.ENOTDIR
			case "permission":
				parent := l.Path
				l.Path = filepath.Join(parent, "child")
				if err := os.Mkdir(l.Path, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(parent, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.Chmod(parent, 0700) })
				if _, err := os.Stat(l.Path); !errors.Is(err, os.ErrPermission) {
					t.Skipf("OS does not deny access under this user: %v", err)
				}
				cause = os.ErrPermission
			}
			err := a.CheckNavigation(l)
			var missing *MissingLocation
			if errors.As(err, &missing) != (kind == "missing") {
				t.Fatalf("incorrect cleanup eligibility: %v", err)
			}
			if (err == nil) != (kind == "directory") || cause != nil && !errors.Is(err, cause) {
				t.Fatalf("directory check lost its error identity: %v (want %v)", err, cause)
			}
		})
	}
}

func TestMissingCleanupRechecksIdentityAndFilesystem(t *testing.T) {
	for _, change := range []string{"restored", "file", "ENOTDIR", "permission", "path", "removed", "cancelled context"} {
		t.Run(change, func(t *testing.T) {
			a, l := navigationApp(t)
			ctx := context.Background()
			if err := a.Store.Visit(ctx, l.ID, l.Path); err != nil {
				t.Fatal(err)
			}
			parent := filepath.Join(t.TempDir(), "parent")
			if err := os.MkdirAll(filepath.Join(parent, "child"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := a.Edit(ctx, l, filepath.Join(parent, "child")); err != nil {
				t.Fatal(err)
			}
			l, _ = a.Store.Resolve(ctx, string(l.ID))
			missing := missingLocation(t, a, l)
			var cause error
			want := ""
			switch change {
			case "restored":
				if err := os.Mkdir(l.Path, 0700); err != nil {
					t.Fatal(err)
				}
				want = "volvió a estar disponible"
			case "file":
				if err := os.WriteFile(l.Path, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			case "ENOTDIR":
				if err := os.Remove(parent); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(parent, nil, 0600); err != nil {
					t.Fatal(err)
				}
				cause = syscall.ENOTDIR
			case "permission":
				if err := os.Chmod(parent, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.Chmod(parent, 0700) })
				if _, err := os.Stat(l.Path); !errors.Is(err, os.ErrPermission) {
					t.Skipf("OS does not deny access under this user: %v", err)
				}
				cause = os.ErrPermission
			case "path":
				if err := a.Edit(ctx, l, t.TempDir()); err != nil {
					t.Fatal(err)
				}
				want = "la ubicación cambió"
			case "removed":
				if err := a.Store.Remove(ctx, l.ID); err != nil {
					t.Fatal(err)
				}
				cause = store.ErrNotFound
			case "cancelled context":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
				cause = context.Canceled
			}
			err := missing.Remove(ctx)
			if err == nil || !strings.Contains(err.Error(), want) || cause != nil && !errors.Is(err, cause) {
				t.Fatalf("cleanup did not refuse %s: %v", change, err)
			}
			if change != "removed" {
				got, err := a.Store.Resolve(context.Background(), string(l.ID))
				if err != nil || got.Visits != 1 {
					t.Fatalf("refusal changed registration or visits: %+v %v", got, err)
				}
			}
		})
	}
}

func TestMissingCleanupPreservesSiblingsAndIgnoredWorktrees(t *testing.T) {
	ctx := context.Background()
	a, main := navigationApp(t)
	gitTest(t, main.Path, "init", "-q", "-b", "main")
	gitTest(t, main.Path, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "initial")
	worktree := filepath.Join(t.TempDir(), "worktree")
	gitTest(t, main.Path, "worktree", "add", "-q", "-b", "feature", worktree)
	if _, err := a.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	l, err := a.Store.Resolve(ctx, worktree)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Store.Visit(ctx, main.ID, main.Path); err != nil {
		t.Fatal(err)
	}
	parked := filepath.Join(t.TempDir(), "parked")
	if err := os.Rename(worktree, parked); err != nil {
		t.Fatal(err)
	}
	var missing *MissingLocation
	if err := a.CheckNavigation(l); !errors.As(err, &missing) {
		t.Fatal(err)
	}
	copy := missing.Location()
	copy.ID, copy.Path = main.ID, main.Path
	if err := missing.Remove(ctx); err != nil {
		t.Fatal(err)
	}
	if err := missing.Remove(ctx); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("repeat cleanup lost missing-ID error: %v", err)
	}
	if err := os.Rename(parked, worktree); err != nil {
		t.Fatal(err)
	}
	r, err := a.Refresh(ctx)
	if err != nil || r.Added != 0 {
		t.Fatalf("refresh resurrected ignored worktree: %+v %v", r, err)
	}
	locations, err := a.Store.List(ctx)
	if err != nil || len(locations) != 1 || locations[0].ID != main.ID || locations[0].Visits != 1 {
		t.Fatalf("cleanup changed sibling or visits: %+v %v", locations, err)
	}
	if _, err := os.Stat(worktree); err != nil {
		t.Fatal(err)
	}
	if r, err := a.Add(ctx, AddInput{Path: worktree, Discover: true}); err != nil || r.Added != 1 {
		t.Fatalf("explicit re-add failed: %+v %v", r, err)
	}
}

func TestMissingCleanupRejectsUnissuedTarget(t *testing.T) {
	for _, missing := range []*MissingLocation{nil, {}} {
		if err := missing.Remove(context.Background()); err == nil {
			t.Fatal("unissued target allowed cleanup")
		}
	}
}
