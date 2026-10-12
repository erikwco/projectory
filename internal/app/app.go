package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/erikwco/projectory/internal/catalog"
	"github.com/erikwco/projectory/internal/inspect"
	"github.com/erikwco/projectory/internal/store"
)

type App struct{ Store *store.Store }
type AddInput struct {
	Path, Name, Description, Label, Project string
	Tags                                    []string
	Discover                                bool
}

func (a *App) Add(ctx context.Context, in AddInput) (store.Registered, error) {
	source, isRoot, err := inspect.Directory(ctx, in.Path)
	if err != nil {
		return store.Registered{}, err
	}
	source.Label = strings.TrimSpace(in.Label)
	if source.Label == "" && source.Kind == catalog.Worktree {
		source.Label = source.Branch
	}
	if source.Label == "" {
		if in.Project != "" || source.Kind == catalog.Worktree {
			source.Label = filepath.Base(source.Path)
		} else {
			source.Label = "principal"
		}
	}
	var project catalog.ProjectID
	if in.Project != "" {
		project, err = a.Store.Project(ctx, in.Project)
		if err != nil {
			return store.Registered{}, err
		}
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = filepath.Base(source.Path)
	}
	if strings.ContainsAny(name, "\x00\n\r\t") {
		return store.Registered{}, fmt.Errorf("el nombre no puede contener caracteres de control")
	}
	candidates := []catalog.Candidate{source}
	if in.Discover && isRoot && source.Repository != nil {
		others, err := inspect.Worktrees(ctx, source)
		if err != nil {
			return store.Registered{}, err
		}
		candidates = append(candidates, others...)
	}
	return a.Store.Register(ctx, store.Registration{ProjectID: project, Name: name, Description: strings.TrimSpace(in.Description), Tags: in.Tags, Locations: candidates, AutoGroup: isRoot})
}

type RefreshResult struct {
	Checked, Added, Missing int
	Warnings                []string
}

func (a *App) Edit(ctx context.Context, l catalog.Location, newPath string) error {
	l.Project.Name = strings.TrimSpace(l.Project.Name)
	if l.Project.Name == "" || strings.ContainsAny(l.Project.Name, "\x00\n\r\t") {
		return fmt.Errorf("indica un nombre de proyecto válido")
	}
	var relocation *catalog.Candidate
	if newPath != l.Path {
		c, _, err := inspect.Directory(ctx, newPath)
		if err != nil {
			return err
		}
		relocation = &c
	}
	return a.Store.Edit(ctx, l, relocation)
}

func (a *App) Refresh(ctx context.Context) (RefreshResult, error) {
	var result RefreshResult
	locations, err := a.Store.List(ctx)
	if err != nil {
		return result, err
	}
	seen := map[string]bool{}
	for _, l := range locations {
		if err = ctx.Err(); err != nil {
			return result, err
		}
		if !l.Available {
			result.Missing++
			continue
		}
		c, root, err := inspect.Directory(ctx, l.Path)
		if err != nil {
			result.Warnings = append(result.Warnings, l.Path+": "+err.Error())
			continue
		}
		if err = a.Store.Observe(ctx, l.ID, c); err != nil {
			return result, err
		}
		result.Checked++
		if c.Repository == nil || !root || seen[c.Repository.CommonDir] {
			continue
		}
		seen[c.Repository.CommonDir] = true
		others, err := inspect.Worktrees(ctx, c)
		if err != nil {
			result.Warnings = append(result.Warnings, err.Error())
			continue
		}
		r, err := a.Store.Register(ctx, store.Registration{ProjectID: l.Project.ID, Locations: append([]catalog.Candidate{c}, others...)})
		if err != nil {
			return result, err
		}
		result.Added += r.Added
		for _, path := range r.Skipped {
			result.Warnings = append(result.Warnings, "ya pertenece a otro proyecto: "+path)
		}
	}
	return result, nil
}
