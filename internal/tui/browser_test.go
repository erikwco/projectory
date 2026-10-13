package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/erikwco/projectory/internal/catalog"
)

func press(code rune) tea.KeyPressMsg       { return tea.KeyPressMsg{Code: code} }
func update(b Browser, msg tea.Msg) Browser { model, _ := b.Update(msg); return model.(Browser) }
func typeQuery(b Browser, text string) Browser {
	for _, r := range text {
		b = update(b, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return b
}

func sampleBrowser(t *testing.T) Browser {
	return NewBrowser(context.Background(), nil, []catalog.Location{
		{ID: "one", Project: catalog.Project{ID: "p1", Name: "Uno", Description: "Una descripción", Tags: []string{"personal"}}, Path: t.TempDir(), Label: "principal", Kind: catalog.Directory, Available: true},
		{ID: "two", Project: catalog.Project{ID: "p2", Name: "Dos"}, Path: t.TempDir(), Label: "principal", Kind: catalog.Directory, Available: true},
	}, true, "", "", "name", SettingsScreen{})
}

func TestSearchFiltersWhileTypingBeforeEnterCanNavigate(t *testing.T) {
	for _, dashboard := range []bool{false, true} {
		name := "list"
		if dashboard {
			name = "dashboard"
		}
		t.Run(name, func(t *testing.T) {
			b := sampleBrowser(t)
			b.Dashboard = dashboard
			b = update(b, press('/'))
			if b.Mode != searching || !b.Query.Focused() {
				t.Fatal("slash didn't focus search")
			}
			b = typeQuery(b, "Uno")
			if len(b.Visible) != 1 || b.Visible[0].Project.ID != "p1" || b.Mode != searching || b.Choice != nil {
				t.Fatal("typing didn't filter immediately while keeping search focused")
			}
			b = typeQuery(b, "x")
			if len(b.Visible) != 0 || !strings.Contains(b.View().Content, "No hay proyectos") {
				t.Fatal("unmatched query didn't empty the list immediately")
			}
			b = update(b, press(tea.KeyBackspace))
			if len(b.Visible) != 1 || b.Visible[0].Project.ID != "p1" {
				t.Fatal("backspace didn't restore the matching row")
			}
			b = update(b, press(tea.KeyEnter))
			if b.Mode != browsing || b.Query.Focused() || len(b.Visible) != 1 || b.Choice != nil {
				t.Fatal("first Enter should only return focus to the filtered list")
			}
			b = update(b, press(tea.KeyEnter))
			if b.Choice == nil || b.Choice.ID != "one" {
				t.Fatal("second Enter didn't select the filtered row")
			}
		})
	}
}

func TestEscapeRestoresCommittedSearch(t *testing.T) {
	b := sampleBrowser(t)
	b = update(b, press('/'))
	b = typeQuery(b, "Uno")
	b = update(b, press(tea.KeyEnter))
	b = update(b, press('/'))
	b = update(b, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if len(b.Visible) != 2 {
		t.Fatal("clearing the input didn't restore the list immediately")
	}
	b = update(b, tea.PasteMsg{Content: "Dos"})
	if len(b.Visible) != 1 || b.Visible[0].Project.ID != "p2" {
		t.Fatal("pasting didn't filter immediately")
	}
	b = update(b, press(tea.KeyEscape))
	if b.Applied != "Uno" || b.Query.Value() != "Uno" || len(b.Visible) != 1 || b.Visible[0].Project.ID != "p1" || b.Mode != browsing || b.Query.Focused() {
		t.Fatal("cancel changed the committed filter")
	}
	b = update(b, press(tea.KeyEscape))
	if len(b.Visible) != 2 || b.Applied != "" {
		t.Fatal("Escape failed to clear filter")
	}
}

func TestSearchArrowsFocusAndMoveThroughFilteredProjects(t *testing.T) {
	for _, auto := range []bool{false, true} {
		for _, dashboard := range []bool{false, true} {
			b := browserWithSettings(t, auto, dashboard)
			if !auto {
				b = update(b, press('/'))
			}
			b = typeQuery(b, "o") // Both Dos and Uno match, in that order.
			b = update(b, press(tea.KeyDown))
			if b.Mode != browsing || b.Query.Focused() || b.Selected != 1 || b.Applied != "o" || b.Choice != nil {
				t.Fatal("Down should commit the live filter, focus the list, and select its next project")
			}
			b = update(b, press('/'))
			b = update(b, press(tea.KeyUp))
			if b.Mode != browsing || b.Query.Focused() || b.Selected != 0 || b.Applied != "o" {
				t.Fatal("Up should resume movement from the current selection")
			}
			b = update(b, press(tea.KeyEnter))
			if b.Choice == nil || b.Choice.ID != "two" {
				t.Fatal("Enter after an arrow should open the selected project")
			}
		}
	}
}

func TestSearchTabFocusesFirstResultBeforeOpeningDetail(t *testing.T) {
	for _, auto := range []bool{false, true} {
		for _, dashboard := range []bool{false, true} {
			b := browserWithSettings(t, auto, dashboard)
			if !auto {
				b = update(b, press('/'))
			}
			b = typeQuery(b, "o") // Both Dos and Uno match, in that order.
			b = update(b, press(tea.KeyDown))
			b = update(b, press('/'))
			b = update(b, press(tea.KeyTab))
			if b.Mode != browsing || b.Query.Focused() || b.Applied != "o" || b.Query.Value() != "o" || len(b.Visible) != 2 || b.Selected != 0 {
				t.Fatal("Tab should commit the live filter and focus its first result, like Enter")
			}
			if b.Choice != nil || b.Picker != nil {
				t.Fatal("first Tab selected a location")
			}
			b = update(b, press(tea.KeyTab))
			if b.Mode != details || b.Selected != 0 {
				t.Fatal("Tab from the list should open the detail of the focused result")
			}
			b = update(b, press(tea.KeyTab))
			b = update(b, press(tea.KeyEnter))
			if b.Choice == nil || b.Choice.ID != "two" {
				t.Fatal("Enter after Tab should open the first result")
			}
		}
	}
}

func TestSearchTabWithoutResultsOnlyFocusesList(t *testing.T) {
	for _, auto := range []bool{false, true} {
		b := browserWithSettings(t, auto, false)
		if !auto {
			b = update(b, press('/'))
		}
		b = typeQuery(b, "missing")
		b = update(b, press(tea.KeyTab))
		if b.Mode != browsing || b.Query.Focused() || b.Applied != "missing" {
			t.Fatal("Tab on an empty search didn't focus the list")
		}
		b = update(b, press(tea.KeyTab))
		b = update(b, press(tea.KeyEnter))
		if b.Mode != browsing || b.Applied != "missing" || len(b.Visible) != 0 || b.Choice != nil {
			t.Fatal("Tab on an empty search opened a detail or selected a location")
		}
	}
}

func TestAutomaticSearchResumesTypingAfterTab(t *testing.T) {
	b := browserWithSettings(t, true, false)
	b = typeQuery(b, "o")
	b = update(b, press(tea.KeyTab))
	if b.Mode != browsing {
		t.Fatal("Tab didn't focus the list")
	}
	b = typeQuery(b, "s")
	if b.Mode != searching || !b.Query.Focused() || b.Query.Value() != "os" || len(b.Visible) != 1 || b.Visible[0].Project.ID != "p2" {
		t.Fatal("typing after Tab did not resume the committed query")
	}
}

func TestSearchArrowsHandleEmptyResultsAndLeaveCursorKeysInInput(t *testing.T) {
	b := browserWithSettings(t, true, false)
	b = typeQuery(b, "missing")
	for _, arrow := range []rune{tea.KeyLeft, tea.KeyRight} {
		b = update(b, press(arrow))
		if b.Mode != searching || !b.Query.Focused() {
			t.Fatal("horizontal arrows must continue editing the query")
		}
	}
	b = update(b, press(tea.KeyDown))
	b = update(b, press(tea.KeyEnter))
	if b.Mode != browsing || b.Selected != 0 || len(b.Visible) != 0 || b.Choice != nil || b.Applied != "missing" {
		t.Fatal("moving out of an empty search lost its query or selected a nonexistent project")
	}
}

func TestAutomaticSearchResumesTypingAfterListNavigation(t *testing.T) {
	for _, dashboard := range []bool{false, true} {
		b := browserWithSettings(t, true, dashboard)
		b = typeQuery(b, "o")
		b = update(b, press(tea.KeyDown))
		b = typeQuery(b, "s")
		if b.Mode != searching || !b.Query.Focused() || b.Query.Value() != "os" || len(b.Visible) != 1 || b.Visible[0].Project.ID != "p2" || b.Selected != 0 {
			t.Fatal("typing from the list did not preserve the query and append the first character")
		}
		b = update(b, press(tea.KeyDown))
		b = update(b, press(tea.KeyBackspace))
		if b.Mode != searching || b.Query.Value() != "o" || len(b.Visible) != 2 {
			t.Fatal("backspace from the list did not resume query editing")
		}
		b = update(b, press(tea.KeyDown))
		b = update(b, tea.PasteMsg{Content: "s"})
		if b.Mode != searching || b.Query.Value() != "os" || len(b.Visible) != 1 || b.Choice != nil {
			t.Fatal("pasting from the list did not resume filtering")
		}
	}
}

func TestAutomaticSearchTreatsLetterShortcutsAsQueryText(t *testing.T) {
	for _, r := range "qjkgGaedroñ " {
		t.Run(string(r), func(t *testing.T) {
			b := browserWithSettings(t, true, true)
			b = update(b, press(tea.KeyDown))
			b = typeQuery(b, string(r))
			if b.Mode != searching || b.Query.Value() != string(r) || b.Busy || b.Choice != nil {
				t.Fatal("printable text triggered a list action instead of filtering")
			}
		})
	}
}

func TestAutomaticSearchPreservesDashboardActionsWithControlShortcuts(t *testing.T) {
	for _, action := range []struct {
		key  rune
		mode mode
	}{{'a', adding}, {'e', editing}, {'d', confirming}} {
		b := browserWithSettings(t, true, true)
		b = update(b, press(tea.KeyDown))
		b = update(b, tea.KeyPressMsg{Code: action.key, Mod: tea.ModCtrl})
		if b.Mode != action.mode || b.Query.Value() != "" {
			t.Fatalf("Ctrl+%c failed to open the dashboard action", action.key)
		}
	}
	b := browserWithSettings(t, true, false)
	b = update(b, press(tea.KeyDown))
	b = update(b, tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	if b.Order != "added" || b.Mode != browsing {
		t.Fatal("Ctrl+O failed to change the list order")
	}
	_, quit := b.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if quit == nil {
		t.Fatal("Ctrl+C did not exit the automatic-search list")
	}
	model, refresh := b.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	if refresh == nil || !model.(Browser).Busy {
		t.Fatal("Ctrl+R failed to start refreshing the list")
	}
	manual := browserWithSettings(t, false, true)
	manual = typeQuery(manual, "j")
	if manual.Selected != 1 || manual.Mode != browsing {
		t.Fatal("manual mode lost its letter navigation shortcut")
	}
	manual = typeQuery(manual, "e")
	if manual.Mode != editing {
		t.Fatal("manual mode lost its letter editing shortcut")
	}
}

func TestLiveSearchKeepsFocusAndTagOnReload(t *testing.T) {
	b := sampleBrowser(t)
	b.Tag = "personal"
	b.filter()
	b = update(b, press('/'))
	b = typeQuery(b, "Dos")
	if len(b.Visible) != 0 {
		t.Fatal("live search ignored the tag filter")
	}
	b = update(b, loaded{Locations: b.Locations})
	if b.Mode != searching || !b.Query.Focused() || len(b.Visible) != 0 {
		t.Fatal("background reload changed search focus or results")
	}
	b = update(b, press(tea.KeyEnter))
	b = update(b, press(tea.KeyEnter))
	if b.Choice != nil {
		t.Fatal("Enter selected a location from an empty result")
	}
}

func TestViewsFitTerminalSizesAndPreserveSelection(t *testing.T) {
	b := sampleBrowser(t)
	b.Selected = 1
	id, _ := b.current()
	for _, size := range [][2]int{{160, 45}, {110, 25}, {80, 24}, {60, 16}, {40, 12}, {24, 8}, {12, 4}} {
		b = update(b, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		current, _ := b.current()
		if current.ID != id.ID {
			t.Fatal("resize changed selection")
		}
		view := b.View()
		lines := strings.Split(view.Content, "\n")
		if len(lines) > size[1] {
			t.Fatalf("height overflow at %v: %d", size, len(lines))
		}
		for _, line := range lines {
			if lipgloss.Width(line) > size[0] {
				t.Fatalf("width overflow at %v: %q", size, line)
			}
		}
	}
}

func TestMissingLocationCannotBeSelected(t *testing.T) {
	b := sampleBrowser(t)
	b.Visible[0].Locations[0].Path = t.TempDir() + "/missing"
	b = update(b, press(tea.KeyEnter))
	if b.Choice != nil || b.Notice == "" {
		t.Fatal("missing directory was selected")
	}
}

func TestTerminalTextCannotInjectControlSequences(t *testing.T) {
	got := Clean("name\x1b[2J\x1b]0;hijack\a\nnext")
	if strings.ContainsAny(got, "\x1b\a\n") {
		t.Fatalf("control sequence leaked: %q", got)
	}
}

func TestDashboardShowsOneRowPerProject(t *testing.T) {
	b := sampleBrowser(t)
	b.Locations = append(b.Locations, catalog.Location{ID: "three", Project: b.Locations[0].Project, Path: t.TempDir(), Label: "copy", Kind: catalog.Directory, Available: true})
	b.filter()
	if len(b.Visible) != 2 || len(b.Visible[1].Locations) != 2 {
		t.Fatal("project locations were not grouped into one row")
	}
	for _, size := range [][2]int{{80, 24}, {40, 12}, {24, 8}} {
		b = update(b, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for i := range b.Visible {
			b.Selected = i
			if !strings.Contains(b.View().Content, "> ") {
				t.Fatalf("selected row hidden at %v index %d", size, i)
			}
		}
	}
}

func TestDetailViewKeepsSelectionWhileScrolling(t *testing.T) {
	b := sampleBrowser(t)
	selected := b.Selected
	b = update(b, tea.WindowSizeMsg{Width: 35, Height: 12})
	b = update(b, press(tea.KeyTab))
	b = update(b, press(tea.KeyDown))
	if b.Mode != details || b.Selected != selected {
		t.Fatal("detail scrolling changed selection")
	}
	if !strings.Contains(b.View().Content, "Detalle") {
		t.Fatal("detail view missing")
	}
	b = update(b, press(tea.KeyEscape))
	if b.Mode != browsing {
		t.Fatal("detail did not close")
	}
}
