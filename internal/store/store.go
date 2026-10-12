package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/erikwco/projectory/internal/catalog"
	_ "modernc.org/sqlite"
)

type Store struct {
	db   *sql.DB
	Path string
}

var ErrNotFound = errors.New("ubicación no encontrada")

func DefaultPath() (string, error) {
	if dir := os.Getenv("PROJECTORY_DATA_DIR"); dir != "" {
		return filepath.Join(dir, "catalog.db"), nil
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "projectory", "catalog.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "projectory", "catalog.db"), nil
	}
	if runtime.GOOS == "windows" {
		dir, err := os.UserConfigDir()
		return filepath.Join(dir, "projectory", "catalog.db"), err
	}
	return filepath.Join(home, ".local", "share", "projectory", "catalog.db"), nil
}

func Open(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Set("_txlock", "immediate")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, Path: abs}
	if err = s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > 1 {
		return fmt.Errorf("el catálogo usa una versión más nueva; actualiza Projectory")
	}
	if version == 1 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`
CREATE TABLE IF NOT EXISTS projects (
 id TEXT PRIMARY KEY, name TEXT NOT NULL COLLATE NOCASE UNIQUE,
 description TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS repositories (
 id TEXT PRIMARY KEY, common_dir TEXT NOT NULL UNIQUE, remote TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS locations (
 id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 path TEXT NOT NULL UNIQUE, label TEXT NOT NULL DEFAULT '',
 kind TEXT NOT NULL CHECK(kind IN ('directory','repository','worktree')),
 repository_id TEXT REFERENCES repositories(id), is_repo_root INTEGER NOT NULL DEFAULT 0,
 branch TEXT NOT NULL DEFAULT '', head TEXT NOT NULL DEFAULT '',
 technologies TEXT NOT NULL DEFAULT '[]', added_at TEXT NOT NULL,
 visited_at TEXT, visits INTEGER NOT NULL DEFAULT 0 CHECK(visits >= 0)
);
CREATE INDEX IF NOT EXISTS locations_project ON locations(project_id);
CREATE INDEX IF NOT EXISTS locations_repository ON locations(repository_id);
CREATE TABLE IF NOT EXISTS project_tags (
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 tag TEXT NOT NULL, PRIMARY KEY(project_id,tag)
);
CREATE TABLE IF NOT EXISTS ignored_locations (path TEXT PRIMARY KEY);
PRAGMA user_version=1;`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

type Registration struct {
	ProjectID         catalog.ProjectID
	Name, Description string
	Tags              []string
	Locations         []catalog.Candidate
	AutoGroup         bool
}
type Registered struct {
	Primary catalog.LocationID
	Added   int
	Skipped []string
}

func repository(ctx context.Context, tx *sql.Tx, repo *catalog.RepositoryInfo) (any, error) {
	if repo == nil {
		return nil, nil
	}
	id := catalog.RepositoryID(catalog.NewID())
	_, err := tx.ExecContext(ctx, `INSERT INTO repositories(id,common_dir,remote) VALUES(?,?,?) ON CONFLICT(common_dir) DO UPDATE SET remote=excluded.remote`, id, repo.CommonDir, repo.Remote)
	if err != nil {
		return nil, err
	}
	err = tx.QueryRowContext(ctx, `SELECT id FROM repositories WHERE common_dir=?`, repo.CommonDir).Scan(&id)
	return id, err
}

func (s *Store) Register(ctx context.Context, in Registration) (Registered, error) {
	var result Registered
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	project := in.ProjectID
	// Resolve duplicates under the write lock, including two terminals adding
	// the same folder at once. Preserve the original metadata and identity.
	var existingOwner catalog.ProjectID
	err = tx.QueryRowContext(ctx, `SELECT project_id FROM locations WHERE path=?`, in.Locations[0].Path).Scan(&existingOwner)
	if err == nil {
		if project != "" && project != existingOwner {
			return result, errors.New("la ubicación ya pertenece a otro proyecto")
		}
		project = existingOwner
	} else if !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	if project == "" && in.AutoGroup && in.Locations[0].Repository != nil {
		var inferred sql.NullString
		err = tx.QueryRowContext(ctx, `SELECT CASE WHEN count(DISTINCT l.project_id)=1 THEN min(l.project_id) END FROM locations l JOIN repositories r ON l.repository_id=r.id WHERE r.common_dir=? AND l.is_repo_root=1`, in.Locations[0].Repository.CommonDir).Scan(&inferred)
		if err != nil {
			return result, err
		}
		if inferred.Valid {
			project = catalog.ProjectID(inferred.String)
		}
	}
	if project == "" {
		project = catalog.ProjectID(catalog.NewID())
		_, err = tx.ExecContext(ctx, `INSERT INTO projects(id,name,description) VALUES(?,?,?)`, project, in.Name, in.Description)
		if err != nil {
			return result, fmt.Errorf("crear proyecto %q (si ya existe, usa --project): %w", in.Name, err)
		}
		for _, tag := range in.Tags {
			if _, err = tx.ExecContext(ctx, `INSERT INTO project_tags VALUES(?,?)`, project, tag); err != nil {
				return result, err
			}
		}
	}
	for i, c := range in.Locations {
		if i == 0 {
			if _, err = tx.ExecContext(ctx, `DELETE FROM ignored_locations WHERE path=?`, c.Path); err != nil {
				return result, err
			}
		} else {
			var ignored int
			if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM ignored_locations WHERE path=?`, c.Path).Scan(&ignored); err != nil {
				return result, err
			}
			if ignored > 0 {
				continue
			}
		}
		var id catalog.LocationID
		var owner catalog.ProjectID
		err = tx.QueryRowContext(ctx, `SELECT id,project_id FROM locations WHERE path=?`, c.Path).Scan(&id, &owner)
		if err == nil {
			if owner != project {
				result.Skipped = append(result.Skipped, c.Path)
				continue
			}
			if i == 0 {
				result.Primary = id
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return result, err
		}
		repo, err := repository(ctx, tx, c.Repository)
		if err != nil {
			return result, err
		}
		tech, err := json.Marshal(c.Technologies)
		if err != nil {
			return result, err
		}
		id = catalog.LocationID(catalog.NewID())
		_, err = tx.ExecContext(ctx, `INSERT INTO locations(id,project_id,path,label,kind,repository_id,is_repo_root,branch,head,technologies,added_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, id, project, c.Path, c.Label, c.Kind, repo, c.IsRepoRoot, c.Branch, c.Head, string(tech), time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return result, err
		}
		result.Added++
		if i == 0 {
			result.Primary = id
		}
	}
	if result.Primary == "" {
		return result, errors.New("la ubicación ya pertenece a otro proyecto")
	}
	return result, tx.Commit()
}

func (s *Store) Project(ctx context.Context, ref string) (catalog.ProjectID, error) {
	var id catalog.ProjectID
	err := s.db.QueryRowContext(ctx, `SELECT id FROM projects WHERE id=? OR name=? COLLATE NOCASE`, ref, ref).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("no existe el proyecto %q", ref)
	}
	return id, err
}

func (s *Store) List(ctx context.Context) ([]catalog.Location, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT l.id,p.id,p.name,p.description,l.path,l.label,l.kind,r.common_dir,r.remote,l.is_repo_root,l.branch,l.head,l.technologies,l.added_at,l.visited_at,l.visits,
 COALESCE((SELECT json_group_array(tag) FROM (SELECT tag FROM project_tags WHERE project_id=p.id ORDER BY tag)),'[]')
 FROM locations l JOIN projects p ON l.project_id=p.id LEFT JOIN repositories r ON l.repository_id=r.id ORDER BY l.added_at DESC,l.path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []catalog.Location{}
	for rows.Next() {
		var l catalog.Location
		var common, remote, visited sql.NullString
		var tech, added, tags string
		if err = rows.Scan(&l.ID, &l.Project.ID, &l.Project.Name, &l.Project.Description, &l.Path, &l.Label, &l.Kind, &common, &remote, &l.IsRepoRoot, &l.Branch, &l.Head, &tech, &added, &visited, &l.Visits, &tags); err != nil {
			return nil, err
		}
		if common.Valid {
			l.Repository = &catalog.RepositoryInfo{CommonDir: common.String, Remote: remote.String}
		}
		if err = json.Unmarshal([]byte(tech), &l.Technologies); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(tags), &l.Project.Tags); err != nil {
			return nil, err
		}
		l.AddedAt, err = time.Parse(time.RFC3339Nano, added)
		if err != nil {
			return nil, err
		}
		if visited.Valid {
			v, err := time.Parse(time.RFC3339Nano, visited.String)
			if err != nil {
				return nil, err
			}
			l.VisitedAt = &v
		}
		info, err := os.Stat(l.Path)
		l.Available = err == nil && info.IsDir()
		result = append(result, l)
	}
	return result, rows.Err()
}

func (s *Store) Resolve(ctx context.Context, ref string) (catalog.Location, error) {
	locations, err := s.List(ctx)
	if err != nil {
		return catalog.Location{}, err
	}
	path, _ := catalog.NormalizePath(ref, false)
	for _, l := range locations {
		if string(l.ID) == ref || l.Path == path {
			return l, nil
		}
	}
	var matches []catalog.Location
	for _, l := range locations {
		if strings.EqualFold(l.Project.Name, ref) || strings.EqualFold(l.Project.Name+"/"+l.Label, ref) {
			matches = append(matches, l)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return catalog.Location{}, fmt.Errorf("%q tiene varias ubicaciones; usa su ID, ruta o proyecto/etiqueta", ref)
	}
	return catalog.Location{}, fmt.Errorf("%w: %q", ErrNotFound, ref)
}

func (s *Store) Visit(ctx context.Context, id catalog.LocationID, path string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE locations SET visits=visits+1,visited_at=? WHERE id=? AND path=?`, time.Now().UTC().Format(time.RFC3339Nano), id, path)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("la ubicación cambió o ya no está registrada")
	}
	return nil
}

func (s *Store) Remove(ctx context.Context, id catalog.LocationID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO ignored_locations SELECT path FROM locations WHERE id=?`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM locations WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return errors.New("la ubicación ya no existe en el catálogo")
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM projects WHERE NOT EXISTS(SELECT 1 FROM locations WHERE locations.project_id=projects.id)`)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM repositories WHERE NOT EXISTS(SELECT 1 FROM locations WHERE locations.repository_id=repositories.id)`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Edit(ctx context.Context, l catalog.Location, relocation *catalog.Candidate) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE projects SET name=?,description=? WHERE id=?`, l.Project.Name, l.Project.Description, l.Project.ID); err != nil {
		return err
	}
	updated, err := tx.ExecContext(ctx, `UPDATE locations SET label=? WHERE id=?`, l.Label, l.ID)
	if err != nil {
		return err
	}
	if n, _ := updated.RowsAffected(); n != 1 {
		return ErrNotFound
	}
	if relocation != nil {
		repo, e := repository(ctx, tx, relocation.Repository)
		if e != nil {
			return e
		}
		tech, e := json.Marshal(relocation.Technologies)
		if e != nil {
			return e
		}
		if _, err = tx.ExecContext(ctx, `UPDATE locations SET path=?,kind=?,repository_id=?,is_repo_root=?,branch=?,head=?,technologies=? WHERE id=?`, relocation.Path, relocation.Kind, repo, relocation.IsRepoRoot, relocation.Branch, relocation.Head, string(tech), l.ID); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM project_tags WHERE project_id=?`, l.Project.ID); err != nil {
		return err
	}
	for _, tag := range l.Project.Tags {
		if _, err = tx.ExecContext(ctx, `INSERT INTO project_tags VALUES(?,?)`, l.Project.ID, tag); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Observe refreshes derived metadata while preserving identity and usage history.
func (s *Store) Observe(ctx context.Context, id catalog.LocationID, c catalog.Candidate) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	repo, err := repository(ctx, tx, c.Repository)
	if err != nil {
		return err
	}
	tech, err := json.Marshal(c.Technologies)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE locations SET path=?,kind=?,repository_id=?,is_repo_root=?,branch=?,head=?,technologies=? WHERE id=?`, c.Path, c.Kind, repo, c.IsRepoRoot, c.Branch, c.Head, string(tech), id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return errors.New("la ubicación ya no está registrada")
	}
	return tx.Commit()
}

func (s *Store) Backup(ctx context.Context, destination string) error {
	f, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		return errors.New("el archivo de destino ya existe")
	}
	if err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `VACUUM INTO ?`, destination)
	if err != nil {
		_ = os.Remove(destination)
	}
	return err
}
