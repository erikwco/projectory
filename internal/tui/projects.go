package tui

import (
	"sort"
	"strings"
	"time"

	"github.com/erikwco/projectory/internal/catalog"
)

// projectGroup is a view of the catalog. Locations retain their own identities
// and remain the targets of navigation, editing, removal, and visit accounting.
type projectGroup struct {
	Project   catalog.Project
	Locations []catalog.Location
	Preferred catalog.LocationID
	Visits    int64
	AddedAt   time.Time
	VisitedAt *time.Time
}

func groupProjects(locations []catalog.Location, query, tag, order string) ([]projectGroup, int) {
	groups := make([]projectGroup, 0)
	index := make(map[catalog.ProjectID]int)
	for _, l := range locations {
		i, exists := index[l.Project.ID]
		if !exists {
			i = len(groups)
			index[l.Project.ID] = i
			groups = append(groups, projectGroup{Project: l.Project})
		}
		g := &groups[i]
		g.Locations = append(g.Locations, l)
		g.Visits += l.Visits
		if l.AddedAt.After(g.AddedAt) {
			g.AddedAt = l.AddedAt
		}
		if l.VisitedAt != nil && (g.VisitedAt == nil || l.VisitedAt.After(*g.VisitedAt)) {
			g.VisitedAt = l.VisitedAt
		}
	}
	total := len(groups)
	visible := make([]projectGroup, 0, total)
	for _, g := range groups {
		sort.SliceStable(g.Locations, func(i, j int) bool {
			a, b := g.Locations[i], g.Locations[j]
			if catalog.LocationRank(a) != catalog.LocationRank(b) {
				return catalog.LocationRank(a) < catalog.LocationRank(b)
			}
			return a.Path < b.Path
		})
		matches := catalog.Filter(g.Locations, query, tag)
		if len(matches) == 0 {
			continue
		}
		g.Preferred = matches[0].ID
		for _, l := range matches {
			if l.Available {
				g.Preferred = l.ID
				break
			}
		}
		visible = append(visible, g)
	}
	sort.SliceStable(visible, func(i, j int) bool {
		a, b := visible[i], visible[j]
		switch order {
		case "name":
			an, bn := strings.ToLower(a.Project.Name), strings.ToLower(b.Project.Name)
			if an != bn {
				return an < bn
			}
		case "visits":
			if a.Visits != b.Visits {
				return a.Visits > b.Visits
			}
		case "visited":
			if (a.VisitedAt == nil) != (b.VisitedAt == nil) {
				return a.VisitedAt != nil
			}
			if a.VisitedAt != nil && b.VisitedAt != nil && !a.VisitedAt.Equal(*b.VisitedAt) {
				return a.VisitedAt.After(*b.VisitedAt)
			}
		}
		if !a.AddedAt.Equal(b.AddedAt) {
			return a.AddedAt.After(b.AddedAt)
		}
		return a.Project.ID < b.Project.ID
	})
	return visible, total
}

func (g projectGroup) preferredIndex() int {
	for i, l := range g.Locations {
		if l.ID == g.Preferred {
			return i
		}
	}
	return 0
}

func (g projectGroup) available() bool {
	for _, l := range g.Locations {
		if l.Available {
			return true
		}
	}
	return false
}

type locationAction uint8

const (
	openLocation locationAction = iota
	editLocation
	removeLocation
)

type locationPicker struct {
	ProjectID catalog.ProjectID
	Selected  int
	Action    locationAction
}
