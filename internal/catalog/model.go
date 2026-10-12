package catalog

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ProjectID string
type LocationID string
type RepositoryID string

type Kind string

const (
	Directory  Kind = "directory"
	Repository Kind = "repository"
	Worktree   Kind = "worktree"
)

type RepositoryInfo struct {
	CommonDir string `json:"common_dir"`
	Remote    string `json:"remote,omitempty"`
}

// Candidate describes a filesystem observation, not a persistent identity.
type Candidate struct {
	Path         string
	Label        string
	Kind         Kind
	Repository   *RepositoryInfo
	IsRepoRoot   bool
	Branch       string
	Head         string
	Technologies []string
}

type Project struct {
	ID          ProjectID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
}

type Location struct {
	ID           LocationID      `json:"id"`
	Project      Project         `json:"project"`
	Path         string          `json:"path"`
	Label        string          `json:"label"`
	Kind         Kind            `json:"kind"`
	Repository   *RepositoryInfo `json:"repository,omitempty"`
	IsRepoRoot   bool            `json:"is_repository_root"`
	Branch       string          `json:"branch,omitempty"`
	Head         string          `json:"head,omitempty"`
	Technologies []string        `json:"technologies"`
	AddedAt      time.Time       `json:"added_at"`
	VisitedAt    *time.Time      `json:"visited_at,omitempty"`
	Visits       int64           `json:"visits"`
	Available    bool            `json:"available"`
}

func NewID() string { var raw [6]byte; _, _ = rand.Read(raw[:]); return hex.EncodeToString(raw[:]) }

// NormalizePath resolves aliases once at the filesystem boundary. Missing paths
// remain absolute so saved locations survive unmounted disks and deleted folders.
func NormalizePath(path string, mustExist bool) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), "/"))
	}
	if path == "" {
		return "", errors.New("indica una carpeta")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		abs = resolved
	} else if mustExist {
		return "", err
	}
	if mustExist {
		info, err := os.Stat(abs)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", errors.New("la ruta debe ser una carpeta")
		}
	}
	return filepath.Clean(abs), nil
}

func Tags(text string) []string {
	set := map[string]bool{}
	for _, part := range strings.Split(text, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			set[part] = true
		}
	}
	result := make([]string, 0, len(set))
	for tag := range set {
		result = append(result, tag)
	}
	sort.Strings(result)
	return result
}

func Filter(locations []Location, query, tag string) []Location {
	terms := strings.Fields(strings.ToLower(query))
	result := make([]Location, 0, len(locations))
	for _, loc := range locations {
		if tag != "" {
			found := false
			for _, t := range loc.Project.Tags {
				if strings.EqualFold(t, tag) {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		haystack := strings.ToLower(strings.Join([]string{loc.Project.Name, loc.Path, loc.Label, loc.Branch, strings.Join(loc.Project.Tags, " "), strings.Join(loc.Technologies, " ")}, " "))
		match := true
		for _, term := range terms {
			if !strings.Contains(haystack, term) {
				match = false
				break
			}
		}
		if match {
			result = append(result, loc)
		}
	}
	return result
}

func Sort(locations []Location, order string) {
	sort.SliceStable(locations, func(i, j int) bool {
		a, b := locations[i], locations[j]
		switch order {
		case "name":
			if a.Project.Name != b.Project.Name {
				return strings.ToLower(a.Project.Name) < strings.ToLower(b.Project.Name)
			}
		case "visits":
			if a.Visits != b.Visits {
				return a.Visits > b.Visits
			}
		case "visited":
			if a.VisitedAt != nil && b.VisitedAt == nil {
				return true
			}
			if a.VisitedAt == nil && b.VisitedAt != nil {
				return false
			}
			if a.VisitedAt != nil && b.VisitedAt != nil && !a.VisitedAt.Equal(*b.VisitedAt) {
				return a.VisitedAt.After(*b.VisitedAt)
			}
		}
		if !a.AddedAt.Equal(b.AddedAt) {
			return a.AddedAt.After(b.AddedAt)
		}
		return a.Path < b.Path
	})
}
