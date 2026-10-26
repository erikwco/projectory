package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/erikwco/projectory/internal/app"
	"github.com/erikwco/projectory/internal/catalog"
)

func multiLocationBrowser(t *testing.T, dashboard bool) Browser {
	t.Helper()
	project := catalog.Project{ID: "shared", Name: "DemoProject", Description: "Aplicación para la comunidad", Tags: []string{"personal"}}
	return NewBrowser(context.Background(), &app.App{}, []catalog.Location{
		{ID: "worktree", Project: project, Path: t.TempDir(), Label: "feature/payments", Branch: "feature/payments", Kind: catalog.Worktree, Available: true, Visits: 4},
		{ID: "main", Project: project, Path: t.TempDir(), Label: "principal", Branch: "main", Kind: catalog.Repository, Available: true, Visits: 3},
		{ID: "other", Project: catalog.Project{ID: "other", Name: "Projectory"}, Path: t.TempDir(), Label: "principal", Kind: catalog.Directory, Available: true, Visits: 6},
	}, dashboard, "", "", "name", SettingsScreen{})
}

func TestProjectGroupingRetainsEveryLocationAndAggregatesVisits(t *testing.T) {
	b := multiLocationBrowser(t, false)
	if len(b.Visible) != 2 || b.ProjectCount != 2 {
		t.Fatal("each project must have exactly one row")
	}
	g := b.Visible[0]
	if g.Project.ID != "shared" || len(g.Locations) != 2 || g.Visits != 7 || g.Locations[0].ID != "main" {
		t.Fatalf("wrong project summary: %+v", g)
	}
	if _, ok := b.current(); ok {
		t.Fatal("multi-location project must not imply a location target")
	}
	b.Order = "visits"
	b.filter()
	if b.Visible[0].Project.ID != "shared" {
		t.Fatal("project visit order must use the total, not the busiest location")
	}
	// Identical names do not merge unrelated project identities.
	b.Locations[2].Project.Name = "DemoProject"
	b.filter()
	if len(b.Visible) != 2 {
		t.Fatal("project names were used as identities")
	}
}

func TestProjectDatesUseLatestLocation(t *testing.T) {
	b := multiLocationBrowser(t, false)
	now := time.Now()
	old := now.Add(-24 * time.Hour)
	b.Locations[0].AddedAt = now
	b.Locations[1].AddedAt = old
	b.Locations[2].AddedAt = old.Add(time.Hour)
	b.Order = "added"
	b.filter()
	if b.Visible[0].Project.ID != "shared" {
		t.Fatal("newest location should make its project recent")
	}
	b.Locations[0].VisitedAt = &old
	b.Locations[1].VisitedAt = &now
	b.Locations[2].VisitedAt = &old
	b.Order = "visited"
	b.filter()
	if b.Visible[0].Project.ID != "shared" || !b.Visible[0].VisitedAt.Equal(now) {
		t.Fatal("project last visit should use its most recently visited location")
	}
}

func TestMultiLocationNavigationAndSearch(t *testing.T) {
	for _, dashboard := range []bool{false, true} {
		b := multiLocationBrowser(t, dashboard)
		b = update(b, press('/'))
		b = typeQuery(b, "payments")
		if len(b.Visible) != 1 || len(b.Visible[0].Locations) != 2 {
			t.Fatal("branch search should show its project and retain all its locations")
		}
		b = update(b, press(tea.KeyEnter))
		if b.Mode != browsing || b.Choice != nil || b.Picker != nil {
			t.Fatal("search Enter must only return focus to projects")
		}
		b = update(b, press(tea.KeyEnter))
		if b.Mode != choosing || b.Choice != nil {
			t.Fatal("multi-location project must open a chooser")
		}
		l, _ := b.current()
		if l.ID != "worktree" {
			t.Fatal("matching worktree should be preselected")
		}
		b = update(b, press(tea.KeyEscape))
		if b.Picker != nil || b.Mode != browsing || b.Applied != "payments" || b.Visible[b.Selected].Project.ID != "shared" {
			t.Fatal("closing chooser should preserve project selection and search")
		}
		b = update(b, press(tea.KeyEnter))
		b = update(b, press(tea.KeyUp))
		b = update(b, press(tea.KeyEnter))
		if b.Choice == nil || b.Choice.ID != "main" {
			t.Fatal("chooser must navigate to the explicitly chosen location")
		}
	}
}

func TestMultiLocationEditAndRemoveRequireExplicitTarget(t *testing.T) {
	for _, key := range []rune{'e', 'd'} {
		b := multiLocationBrowser(t, true)
		b = update(b, press(key))
		if b.Mode != choosing || b.Picker == nil {
			t.Fatal("administration must ask which location to change")
		}
		b = update(b, press(tea.KeyDown))
		b = update(b, press(tea.KeyEnter))
		if key == 'e' {
			if b.Mode != editing || b.Editing.ID != "worktree" || b.Form.Values()[4] != b.Editing.Path {
				t.Fatal("edit targeted the wrong location")
			}
		} else if b.Mode != confirming || b.Removing.ID != "worktree" || !strings.Contains(b.View().Content, "feature/payments") {
			t.Fatal("removal confirmation must identify the chosen location")
		}
		b = update(b, press(tea.KeyEscape))
		if b.Mode != choosing || b.Picker.Selected != 1 {
			t.Fatal("cancel should return to the selected location")
		}
	}
}

func TestChooserRefreshPreservesIdentityAndHandlesRemovedLocations(t *testing.T) {
	b := multiLocationBrowser(t, true)
	b = update(b, press(tea.KeyEnter))
	b = update(b, press(tea.KeyDown))
	// Removing the primary location moves the selected worktree to index zero.
	b = update(b, loaded{Locations: []catalog.Location{b.Locations[0], b.Locations[2]}})
	l, ok := b.current()
	if b.Mode != choosing || !ok || l.ID != "worktree" || b.Picker.Selected != 0 {
		t.Fatal("refresh didn't preserve selected location by identity")
	}
	// Removing the final location closes the chooser and selects the neighbor.
	b = update(b, loaded{Locations: []catalog.Location{b.Locations[1]}})
	if b.Mode != browsing || b.Picker != nil || len(b.Visible) != 1 || b.Visible[0].Project.ID != "other" {
		t.Fatal("removing a project's final location left a stale chooser")
	}
}

func TestUnavailableLocationRemainsVisibleAndCannotNavigate(t *testing.T) {
	b := multiLocationBrowser(t, false)
	b.Locations[0].Path += "/missing"
	b.Locations[0].Available = false
	b.filter()
	b = update(b, press(tea.KeyEnter))
	b = update(b, press(tea.KeyDown))
	b = update(b, press(tea.KeyEnter))
	if b.Choice != nil || b.Mode != confirming || len(b.Visible[0].Locations) != 2 {
		t.Fatal("unavailable location must remain visible without being selected for navigation")
	}
}

func TestGroupedViewsFitSizesAndRetainSelection(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, size := range [][2]int{{200, 48}, {160, 40}, {110, 25}, {80, 24}, {60, 16}, {40, 12}, {24, 8}, {12, 4}} {
			b := multiLocationBrowser(t, true)
			b.Dark = dark
			b = update(b, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			b.Visible[0].Project.Name = strings.Repeat("Proyecto 日本語 ", 20)
			for _, key := range []rune{0, tea.KeyEnter, tea.KeyTab, tea.KeyEscape, 'd'} {
				if key != 0 {
					b = update(b, press(key))
				}
				view := b.View().Content
				if lipgloss.Height(view) > size[1] {
					t.Fatalf("height overflow at %v mode %v", size, b.Mode)
				}
				for _, line := range strings.Split(view, "\n") {
					if lipgloss.Width(line) > size[0] {
						t.Fatalf("width overflow at %v mode %v: %q", size, b.Mode, line)
					}
				}
			}
		}
	}
}

func TestUndersizedTerminalBlocksHiddenActions(t *testing.T) {
	for _, mode := range []mode{browsing, searching, choosing, details, confirming, adding, editing} {
		b := multiLocationBrowser(t, true)
		b = update(b, press(tea.KeyEnter))
		b.Mode = mode
		b.Removing = b.Locations[0]
		b = update(b, tea.WindowSizeMsg{Width: 12, Height: 4})
		for _, key := range []rune{tea.KeyEnter, 'e', 'd', 's', 'y', tea.KeyDown, tea.KeyEscape} {
			model, cmd := b.Update(press(key))
			after := model.(Browser)
			if cmd != nil || after.Mode != mode || after.Choice != nil || after.Busy || after.Selected != b.Selected || after.Picker.Selected != b.Picker.Selected {
				t.Fatalf("hidden action %v executed in mode %v", key, mode)
			}
		}
		_, cmd := b.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
		if cmd != nil {
			t.Fatal("hidden form could be submitted")
		}
		if !strings.Contains(b.View().Content, "q salir") {
			t.Fatal("fallback must advertise its available action")
		}
		_, cmd = b.Update(press('q'))
		if cmd == nil {
			t.Fatal("quit should remain available")
		}
		b = update(b, tea.WindowSizeMsg{Width: 80, Height: 24})
		if b.Mode != mode || b.Width != 80 {
			t.Fatal("resize should restore the existing interaction")
		}
	}
}
