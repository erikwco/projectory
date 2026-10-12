package store

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/erikwco/projectory/internal/catalog"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func register(t *testing.T, s *Store, name, path string) Registered {
	t.Helper()
	r, err := s.Register(context.Background(), Registration{Name: name, Tags: []string{"personal", "web"}, Locations: []catalog.Candidate{{Path: path, Kind: catalog.Directory, Technologies: []string{"Go"}}}})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestPersistenceAtomicVisitsAndBackup(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	path := t.TempDir()
	r := register(t, s, "Shop", path)
	other, err := Open(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	var wg sync.WaitGroup
	failures := make(chan error, 30)
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			db := s
			if i%2 == 0 {
				db = other
			}
			failures <- db.Visit(ctx, r.Primary, path)
		}(i)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := other.Resolve(ctx, string(r.Primary))
	if err != nil {
		t.Fatal(err)
	}
	if got.Visits != 30 || got.VisitedAt == nil || len(got.Project.Tags) != 2 || !got.Available {
		t.Fatalf("bad persistence: %+v", got)
	}
	if err = s.Visit(ctx, r.Primary, path+"-wrong"); err == nil {
		t.Fatal("accepted visit to a different path")
	}
	backup := filepath.Join(t.TempDir(), "backup.db")
	if err = s.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	copy, err := Open(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer copy.Close()
	saved, err := copy.Resolve(ctx, string(r.Primary))
	if err != nil || saved.Visits != 30 {
		t.Fatalf("backup mismatch: %+v %v", saved, err)
	}
	if err = s.Backup(ctx, backup); err == nil {
		t.Fatal("backup overwrote an existing file")
	}
}

func TestEditRollsBackMetadataWhenPathConflicts(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	one, two := t.TempDir(), t.TempDir()
	r := register(t, s, "One", one)
	register(t, s, "Two", two)
	l, err := s.Resolve(ctx, string(r.Primary))
	if err != nil {
		t.Fatal(err)
	}
	l.Project.Name = "Changed"
	err = s.Edit(ctx, l, &catalog.Candidate{Path: two, Kind: catalog.Directory, Technologies: []string{}})
	if err == nil {
		t.Fatal("expected conflicting path error")
	}
	l, err = s.Resolve(ctx, string(r.Primary))
	if err != nil {
		t.Fatal(err)
	}
	if l.Project.Name != "One" || l.Path != one {
		t.Fatalf("partial edit escaped transaction: %+v", l)
	}
}

func TestMissingFoldersStayRegisteredAndRemovalKeepsFiles(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	path := t.TempDir()
	r := register(t, s, "Missing", path)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	locations, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(locations) != 1 || locations[0].Available {
		t.Fatalf("missing location discarded: %+v", locations)
	}
	if err = os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err = s.Remove(ctx, r.Primary); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal("removal touched filesystem:", err)
	}
	locations, err = s.List(ctx)
	if err != nil || len(locations) != 0 {
		t.Fatalf("remove: %v %+v", err, locations)
	}
}

func TestConcurrentRegistrationOfSamePathIsIdempotent(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	other, err := Open(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	path := t.TempDir()
	input := Registration{Name: "Same", Locations: []catalog.Candidate{{Path: path, Kind: catalog.Directory, Technologies: []string{}}}}
	errors := make(chan error, 2)
	for _, db := range []*Store{s, other} {
		go func(db *Store) { _, err := db.Register(ctx, input); errors <- err }(db)
	}
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	locations, err := s.List(ctx)
	if err != nil || len(locations) != 1 {
		t.Fatalf("duplicate registration: %+v %v", locations, err)
	}
}
