// Package inspect reads project manifests and asks Git about repository metadata.
// It never installs dependencies, fetches remotes, or modifies worktrees.
package inspect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/erikwco/projectory/internal/catalog"
)

func git(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	argv := append([]string{"--no-optional-locks", "-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", argv...)
	// A caller's Git environment must not redirect inspection into another repo.
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if !strings.HasPrefix(key, "GIT_") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return strings.TrimSuffix(string(out), "\n"), err
}

func Directory(ctx context.Context, path string) (catalog.Candidate, bool, error) {
	path, err := catalog.NormalizePath(path, true)
	if err != nil {
		return catalog.Candidate{}, false, err
	}
	c := catalog.Candidate{Path: path, Kind: catalog.Directory, Technologies: Technologies(path)}
	root, err := git(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return c, false, err
		}
		return c, false, nil // Git is optional for ordinary folders.
	}
	root, err = catalog.NormalizePath(root, true)
	if err != nil {
		return c, false, err
	}
	common, err := git(ctx, path, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return c, false, fmt.Errorf("leer repositorio Git: %w", err)
	}
	common, err = catalog.NormalizePath(common, true)
	if err != nil {
		return c, false, err
	}
	remote, _ := git(ctx, path, "remote", "get-url", "origin")
	if u, e := url.Parse(remote); e == nil && u.Scheme != "" && u.User != nil {
		u.User = nil
		remote = u.String()
	}
	c.Repository = &catalog.RepositoryInfo{CommonDir: common, Remote: remote}
	c.IsRepoRoot = path == root
	c.Kind = catalog.Repository
	gitDir, err := git(ctx, path, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return c, false, err
	}
	gitDir, err = catalog.NormalizePath(gitDir, true)
	if err != nil {
		return c, false, err
	}
	if gitDir != common {
		c.Kind = catalog.Worktree
	}
	c.Branch, _ = git(ctx, path, "symbolic-ref", "--quiet", "--short", "HEAD")
	c.Head, _ = git(ctx, path, "rev-parse", "--verify", "HEAD")
	return c, path == root, nil
}

type Worktree struct {
	Path, Branch, Head string
	Bare               bool
}

func ParseWorktrees(output string) ([]Worktree, error) {
	var result []Worktree
	var current *Worktree
	for _, field := range strings.Split(output, "\x00") {
		key, value, _ := strings.Cut(field, " ")
		switch key {
		case "worktree":
			if value == "" {
				return nil, fmt.Errorf("Git devolvió un worktree sin ruta")
			}
			result = append(result, Worktree{Path: value})
			current = &result[len(result)-1]
		case "branch":
			if current != nil {
				current.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "HEAD":
			if current != nil {
				current.Head = value
			}
		case "bare":
			if current != nil {
				current.Bare = true
			}
		}
	}
	return result, nil
}

func Worktrees(ctx context.Context, source catalog.Candidate) ([]catalog.Candidate, error) {
	output, err := git(ctx, source.Path, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, fmt.Errorf("listar worktrees: %w", err)
	}
	entries, err := ParseWorktrees(output)
	if err != nil {
		return nil, err
	}
	var result []catalog.Candidate
	for _, entry := range entries {
		if entry.Bare {
			continue
		}
		path, err := catalog.NormalizePath(entry.Path, false)
		if err != nil {
			return nil, err
		}
		if path == source.Path {
			continue
		}
		kind := catalog.Worktree
		label := entry.Branch
		if filepath.Join(path, ".git") == source.Repository.CommonDir {
			kind = catalog.Repository
			label = "principal"
		}
		if label == "" {
			label = filepath.Base(path)
		}
		result = append(result, catalog.Candidate{Path: path, Label: label, Kind: kind, Repository: source.Repository, IsRepoRoot: true, Branch: entry.Branch, Head: entry.Head, Technologies: Technologies(path)})
	}
	return result, nil
}

func Technologies(path string) []string {
	set := map[string]bool{}
	for file, tech := range map[string]string{"go.mod": "Go", "go.work": "Go", "Cargo.toml": "Rust", "Package.swift": "Swift", "pyproject.toml": "Python", "requirements.txt": "Python", "Gemfile": "Ruby", "composer.json": "PHP", "pom.xml": "Java", "build.gradle": "JVM", "mix.exs": "Elixir", "pubspec.yaml": "Dart"} {
		if info, err := os.Stat(filepath.Join(path, file)); err == nil && !info.IsDir() {
			set[tech] = true
		}
	}
	entries, _ := os.ReadDir(path)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".xcodeproj") || strings.HasSuffix(entry.Name(), ".xcworkspace") {
			set["Xcode"] = true
		}
	}
	manifest := filepath.Join(path, "package.json")
	if info, err := os.Stat(manifest); err == nil && info.Size() < 1024*1024 {
		set["Node.js"] = true
		var pkg struct {
			Dependencies    map[string]json.RawMessage
			DevDependencies map[string]json.RawMessage
		}
		if data, err := os.ReadFile(manifest); err == nil && json.Unmarshal(data, &pkg) == nil {
			for name, tech := range map[string]string{"@remix-run/react": "Remix", "@remix-run/dev": "Remix", "@react-router/dev": "React Router", "next": "Next.js", "react": "React", "vue": "Vue", "nuxt": "Nuxt", "svelte": "Svelte", "@sveltejs/kit": "SvelteKit", "astro": "Astro", "typescript": "TypeScript"} {
				if _, ok := pkg.Dependencies[name]; ok {
					set[tech] = true
				}
				if _, ok := pkg.DevDependencies[name]; ok {
					set[tech] = true
				}
			}
		}
	}
	result := make([]string, 0, len(set))
	for tech := range set {
		result = append(result, tech)
	}
	sort.Strings(result)
	return result
}

type Status struct {
	Branch        string
	Changed       int
	Ahead, Behind string
	Err           error
}

func WorkingStatus(ctx context.Context, path string) Status {
	out, err := git(ctx, path, "status", "--porcelain=v2", "--branch", "-z", "--untracked-files=normal")
	if err != nil {
		return Status{Err: err}
	}
	var status Status
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		switch {
		case strings.HasPrefix(field, "# branch.head "):
			status.Branch = strings.TrimPrefix(field, "# branch.head ")
		case strings.HasPrefix(field, "# branch.ab "):
			parts := strings.Fields(field)
			if len(parts) == 4 {
				status.Ahead = parts[2]
				status.Behind = parts[3]
			}
		case strings.HasPrefix(field, "2 "):
			status.Changed++
			i++ // Rename has a second NUL-delimited pathname.
		case strings.HasPrefix(field, "1 "), strings.HasPrefix(field, "u "), strings.HasPrefix(field, "? "):
			status.Changed++
		}
	}
	return status
}
