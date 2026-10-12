package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/erikwco/projectory/internal/config"
)

func browserWithSettings(t *testing.T, enabled, dashboard bool) Browser {
	t.Helper()
	s := NewSettings(filepath.Join(t.TempDir(), "settings.json"), config.Settings{SearchOnOpen: enabled})
	return NewBrowser(context.Background(), nil, sampleBrowser(t).Locations, dashboard, "", "", "name", s)
}

func TestSearchOnOpenFiltersWithoutSlashAndKeepsTwoStepEnter(t *testing.T) {
	for _, dashboard := range []bool{false, true} {
		b := browserWithSettings(t, true, dashboard)
		if b.Mode != searching || !b.Query.Focused() {
			t.Fatal("search-on-open did not focus the input")
		}
		b = typeQuery(b, "Uno")
		if len(b.Visible) != 1 || b.Visible[0].Project.ID != "p1" || b.Choice != nil {
			t.Fatal("typing without slash did not filter projects")
		}
		b = update(b, press(tea.KeyEnter))
		if b.Mode != browsing || b.Query.Focused() || b.Choice != nil {
			t.Fatal("first Enter should only return focus to the list")
		}
		b = update(b, press(tea.KeyEnter))
		if b.Choice == nil || b.Choice.ID != "one" {
			t.Fatal("second Enter did not select the filtered project")
		}
		manual := browserWithSettings(t, false, dashboard)
		manual = typeQuery(manual, "Uno")
		if manual.Mode != browsing || manual.Query.Value() != "" || len(manual.Visible) != 2 {
			t.Fatal("disabled preference changed manual search behavior")
		}
	}
}

func TestSettingsTogglePersistsAndReturnsToCurrentSearch(t *testing.T) {
	b := browserWithSettings(t, true, false)
	b = typeQuery(b, "Uno")
	b = update(b, press(tea.KeyF2))
	if b.Mode != settingsMode || b.Query.Focused() {
		t.Fatal("settings did not take focus")
	}
	model, cmd := b.Update(press(' '))
	b = model.(Browser)
	if cmd == nil || b.Settings.State != settingsSaving || !b.Settings.Values.SearchOnOpen {
		t.Fatal("toggle must save before reporting the new value")
	}
	_, duplicate := b.Update(press(tea.KeyEnter))
	if duplicate != nil {
		t.Fatal("toggle started overlapping saves")
	}
	b = update(b, cmd())
	saved, err := config.Load(b.Settings.Path)
	if err != nil || saved.SearchOnOpen || b.Settings.Values.SearchOnOpen || b.Settings.Notice != "Guardado" {
		t.Fatalf("toggle was not saved: %+v %v", saved, err)
	}
	b = update(b, press(tea.KeyEscape))
	if b.Mode != searching || !b.Query.Focused() || b.Query.Value() != "Uno" || len(b.Visible) != 1 {
		t.Fatal("return from settings lost the active query or its focus")
	}
	// The preference changes the next launch, not the current interaction.
	b = update(b, press(tea.KeyDown))
	b = typeQuery(b, "x")
	if b.Mode != searching || b.Query.Value() != "Unox" {
		t.Fatal("changing settings modified the current session's typing behavior")
	}
	reopened := NewBrowser(b.Context, nil, b.Locations, false, "", "", "name", NewSettings(b.Settings.Path, saved))
	if reopened.Mode != browsing || reopened.Query.Focused() || reopened.AutoSearch {
		t.Fatal("a fresh browser ignored the saved preference")
	}
}

func TestFailedSettingSaveKeepsPriorValueAndCanRetry(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	s := NewSettings(filepath.Join(blocker, "settings.json"), config.Settings{})
	model, cmd := s.Update(press(tea.KeyEnter))
	s = model.(SettingsScreen)
	model, _ = s.Update(cmd())
	s = model.(SettingsScreen)
	if s.Values.SearchOnOpen || s.Error == "" || s.State != settingsReady {
		t.Fatal("failed save appeared to apply the preference")
	}
	s.Path = filepath.Join(t.TempDir(), "settings.json")
	model, cmd = s.Update(press(tea.KeyEnter))
	s = model.(SettingsScreen)
	model, _ = s.Update(cmd())
	s = model.(SettingsScreen)
	if !s.Values.SearchOnOpen || s.Error != "" {
		t.Fatal("could not retry after a settings save failed")
	}
}

func TestSettingsFitTerminalAndBlockHiddenToggles(t *testing.T) {
	for _, dark := range []bool{true, false} {
		for _, size := range [][2]int{{160, 45}, {80, 24}, {48, 16}, {24, 8}, {12, 4}} {
			s := NewSettings(filepath.Join(t.TempDir(), "settings.json"), config.Settings{})
			s.Width, s.Height, s.Dark = size[0], size[1], dark
			s.Selected = 1
			view := s.View().Content
			if size[0] >= 24 && size[1] >= 8 && (!strings.Contains(view, "Orden predeterminado") || !strings.Contains(view, "registro") || !strings.Contains(view, "Esc")) {
				t.Fatalf("sort setting or exit action hidden at %v: %q", size, view)
			}
			if lipgloss.Height(view) > s.Height {
				t.Fatalf("settings height overflow at %v", size)
			}
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > s.Width {
					t.Fatalf("settings width overflow at %v", size)
				}
			}
			if s.Width < 24 || s.Height < 8 {
				_, cmd := s.Update(press(tea.KeyEnter))
				if cmd != nil {
					t.Fatal("hidden setting could be changed")
				}
			}
		}
	}
}

func TestDefaultSortCyclesBothWaysAndSavesWithoutChangingCurrentSession(t *testing.T) {
	b := browserWithSettings(t, true, false)
	b.Order = "visits"
	b = update(b, press(tea.KeyF2))
	b = update(b, press(tea.KeyDown))
	if b.Settings.Selected != 1 {
		t.Fatal("cannot select default sort")
	}
	for _, step := range []struct {
		key  tea.KeyPressMsg
		want string
	}{{press(tea.KeyLeft), "name"}, {press(tea.KeyRight), "added"}, {press(tea.KeyEnter), "visited"}, {press(' '), "visits"}, {press(tea.KeyRight), "name"}} {
		before := b.Settings.Values.SortOrder()
		model, cmd := b.Update(step.key)
		b = model.(Browser)
		if cmd == nil || b.Settings.Values.SortOrder() != before {
			t.Fatal("preference changed before it was saved")
		}
		b = update(b, cmd())
		saved, err := config.Load(b.Settings.Path)
		if err != nil || saved.SortOrder() != step.want || !saved.SearchOnOpen {
			t.Fatalf("sort preference not persisted: %+v %v", saved, err)
		}
	}
	b = update(b, press(tea.KeyEscape))
	if b.Order != "visits" || b.Mode != searching || !b.Query.Focused() {
		t.Fatal("default order changed the current session or its focus")
	}
}
