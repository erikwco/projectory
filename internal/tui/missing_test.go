package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/erikwco/projectory/internal/app"
	"github.com/erikwco/projectory/internal/catalog"
	"github.com/erikwco/projectory/internal/store"
)

func TestMissingEnterOffersRemovalWithoutSelecting(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	a := &app.App{Store: s}
	path := t.TempDir()
	if _, err := a.Add(ctx, app.AddInput{Path: path, Name: "Tienda", Label: "principal"}); err != nil {
		t.Fatal(err)
	}
	locations, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	b := NewBrowser(ctx, a, locations, false, "", "", "name", SettingsScreen{})
	b = update(b, press(tea.KeyEnter))
	if b.Mode != confirming || !strings.Contains(b.View().Content, "ya no existe") {
		t.Fatalf("Enter on a missing registered path must offer removal confirmation; mode=%v notice=%q", b.Mode, b.Notice)
	}
	if b.Choice != nil {
		t.Fatal("missing-path confirmation selected a location")
	}
}

func cleanupFixture(t *testing.T) (*app.App, []catalog.Location) {
	t.Helper()
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	a := &app.App{Store: s}
	for _, label := range []string{"principal", "copia"} {
		in := app.AddInput{Path: t.TempDir(), Name: "Tienda", Label: label}
		if label == "copia" {
			in.Project = "Tienda"
		}
		if _, err := a.Add(ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	locations, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Visit(ctx, locations[1].ID, locations[1].Path); err != nil {
		t.Fatal(err)
	}
	return a, locations
}

func assertCleanupRecords(t *testing.T, a *app.App, target, sibling catalog.Location, removed bool) {
	t.Helper()
	ctx := context.Background()
	_, err := a.Store.Resolve(ctx, string(target.ID))
	if removed != errors.Is(err, store.ErrNotFound) || !removed && err != nil {
		t.Fatalf("wrong target state: %v", err)
	}
	got, err := a.Store.Resolve(ctx, string(sibling.ID))
	if err != nil || got.Visits != 1 || got.Path != sibling.Path {
		t.Fatalf("sibling changed: %+v %v", got, err)
	}
	if !removed {
		got, err = a.Store.Resolve(ctx, string(target.ID))
		if err != nil || got.Visits != 0 {
			t.Fatalf("cleanup counted a visit: %+v %v", got, err)
		}
	}
}

func TestMatchesMissingCancelAndCleanup(t *testing.T) {
	for _, direct := range []bool{false, true} {
		for _, answer := range []rune{tea.KeyEnter, tea.KeyEscape, 's', 'y'} {
			t.Run(fmt.Sprintf("direct=%v/key=%d", direct, answer), func(t *testing.T) {
				a, locations := cleanupFixture(t)
				l := locations[0]
				if err := os.Remove(l.Path); err != nil {
					t.Fatal(err)
				}
				m := NewMatches(context.Background(), a, "tiend", catalog.Navigation{Locations: locations})
				if direct {
					var missing *app.MissingLocation
					if err := a.CheckNavigation(l); !errors.As(err, &missing) {
						t.Fatal(err)
					}
					m = NewMissingMatches(context.Background(), a, missing)
				}
				m = updateMatches(m, tea.WindowSizeMsg{Width: 100, Height: 24})
				if !direct {
					m = updateMatches(m, press(tea.KeyEnter))
				}
				for _, text := range []string{"ya no existe", l.Project.Name, l.Label, string(l.ID), "Enter/Esc cancelar"} {
					if !strings.Contains(plainView(m), text) {
						t.Fatalf("confirmation hides %q: %s", text, plainView(m))
					}
				}
				model, cmd := m.Update(press(answer))
				m = model.(Matches)
				confirmed := answer == 's' || answer == 'y'
				if confirmed {
					if cmd == nil || !m.busy {
						t.Fatal("confirmation did not start cleanup")
					}
					assertCleanupRecords(t, a, l, locations[1], false)
					for _, key := range []rune{tea.KeyEnter, 's', 'y', tea.KeyDown, tea.KeyEscape} {
						next, duplicate := m.Update(press(key))
						m = next.(Matches)
						if duplicate != nil || m.Choice != nil || !m.busy {
							t.Fatal("input escaped pending cleanup")
						}
					}
					m = updateMatches(m, cmd())
					if m.err != nil || !m.Done || m.completion == "" {
						t.Fatalf("cleanup failed: %+v", m)
					}
				} else if m.Done != direct || m.missing != nil && !direct {
					t.Fatalf("cancel returned to wrong place: %+v", m)
				}
				if m.Choice != nil {
					t.Fatal("cleanup or cancel chose a sibling")
				}
				assertCleanupRecords(t, a, l, locations[1], confirmed)
			})
		}
	}
}

func TestMatchesCleanupRefusesRestoredOrChangedLocation(t *testing.T) {
	for _, change := range []string{"restore", "path", "removed"} {
		t.Run(change, func(t *testing.T) {
			a, locations := cleanupFixture(t)
			l := locations[0]
			if err := os.Remove(l.Path); err != nil {
				t.Fatal(err)
			}
			m := updateMatches(NewMatches(context.Background(), a, "tiend", catalog.Navigation{Locations: locations}), tea.WindowSizeMsg{Width: 100, Height: 24}, press(tea.KeyEnter))
			switch change {
			case "restore":
				if err := os.Mkdir(l.Path, 0700); err != nil {
					t.Fatal(err)
				}
			case "path":
				if err := a.Edit(context.Background(), l, t.TempDir()); err != nil {
					t.Fatal(err)
				}
			case "removed":
				if err := a.Store.Remove(context.Background(), l.ID); err != nil {
					t.Fatal(err)
				}
			}
			model, cmd := m.Update(press('s'))
			if cmd == nil {
				t.Fatal("missing cleanup command")
			}
			m = updateMatches(model.(Matches), cmd())
			if !m.Done || m.Choice != nil || m.err == nil || m.completion != "" {
				t.Fatalf("refusal navigated: %+v", m)
			}
			if change == "restore" && !strings.Contains(m.err.Error(), "volvió a estar disponible") {
				t.Fatal(m.err)
			}
			assertCleanupRecords(t, a, l, locations[1], change == "removed")
		})
	}
}

func TestFreshNavigationIgnoresStaleAvailability(t *testing.T) {
	for _, exists := range []bool{false, true} {
		for _, browser := range []bool{false, true} {
			t.Run(fmt.Sprintf("exists=%v/browser=%v", exists, browser), func(t *testing.T) {
				a, locations := cleanupFixture(t)
				l := locations[0]
				locations = locations[:1]
				if err := os.Remove(l.Path); err != nil {
					t.Fatal(err)
				}
				if exists {
					var err error
					locations[0], err = a.Store.Resolve(context.Background(), string(l.ID))
					if err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(l.Path, 0700); err != nil {
						t.Fatal(err)
					}
				}
				if browser {
					b := update(NewBrowser(context.Background(), a, locations, false, "", "", "name", SettingsScreen{}), press(tea.KeyEnter))
					if (b.Choice != nil) != exists || (b.Mode == confirming) == exists {
						t.Fatal("Browser trusted stale availability")
					}
				} else {
					m := updateMatches(NewMatches(context.Background(), a, "tiend", catalog.Navigation{Locations: locations}), tea.WindowSizeMsg{Width: 100, Height: 24}, press(tea.KeyEnter))
					if (m.Choice != nil) != exists || (m.missing != nil) == exists {
						t.Fatal("Matches trusted stale availability")
					}
				}
			})
		}
	}
}

func TestBrowserMissingCleanupStaysInPicker(t *testing.T) {
	for _, dashboard := range []bool{false, true} {
		for _, answer := range []rune{tea.KeyEnter, tea.KeyEscape, 's', 'y'} {
			t.Run(fmt.Sprintf("dashboard=%v/key=%d", dashboard, answer), func(t *testing.T) {
				a, locations := cleanupFixture(t)
				l := locations[0]
				if err := os.Remove(l.Path); err != nil {
					t.Fatal(err)
				}
				b := NewBrowser(context.Background(), a, locations, dashboard, "Tienda", "", "name", SettingsScreen{})
				b = update(b, press(tea.KeyEnter))
				for i, candidate := range b.Visible[0].Locations {
					if candidate.ID == l.ID {
						b.Picker.Selected = i
					}
				}
				selected := b.Picker.Selected
				b = update(b, press(tea.KeyEnter))
				if b.Mode != confirming || b.Removing.ID != l.ID {
					t.Fatal("missing target was not confirmed")
				}
				model, cmd := b.Update(press(answer))
				b = model.(Browser)
				confirmed := answer == 's' || answer == 'y'
				if confirmed {
					if cmd == nil || !b.Busy {
						t.Fatal("confirmation did not start cleanup")
					}
					assertCleanupRecords(t, a, l, locations[1], false)
					for _, key := range []rune{tea.KeyEnter, 's', 'y', tea.KeyDown, tea.KeyEscape, 'r'} {
						next, duplicate := b.Update(press(key))
						b = next.(Browser)
						if duplicate != nil || b.Choice != nil || !b.Busy {
							t.Fatal("input escaped pending cleanup")
						}
					}
					b = update(b, cmd())
				} else if b.Picker.Selected != selected {
					t.Fatal("cancel moved the selection")
				}
				if b.Choice != nil || b.Busy || b.Mode != choosing || b.Applied != "Tienda" {
					t.Fatalf("cleanup left the picker: %+v", b)
				}
				assertCleanupRecords(t, a, l, locations[1], confirmed)
			})
		}
	}
}

func TestBrowserRestoredCleanupReloadsWithoutOpening(t *testing.T) {
	a, locations := cleanupFixture(t)
	l := locations[0]
	if err := os.Remove(l.Path); err != nil {
		t.Fatal(err)
	}
	b := NewBrowser(context.Background(), a, locations[:1], false, "", "", "name", SettingsScreen{})
	b = update(b, press(tea.KeyEnter))
	if err := os.Mkdir(l.Path, 0700); err != nil {
		t.Fatal(err)
	}
	model, cmd := b.Update(press('s'))
	if cmd == nil {
		t.Fatal("missing cleanup command")
	}
	b = update(model.(Browser), cmd())
	if b.Busy || b.Choice != nil || b.Mode != browsing || !strings.Contains(b.Notice, "volvió a estar disponible") {
		t.Fatalf("restoration did not retain Browser: %+v", b)
	}
	assertCleanupRecords(t, a, l, locations[1], false)
	for _, l := range b.Locations {
		if !l.Available {
			t.Fatal("restoration did not reload availability")
		}
	}
}

func TestHiddenMissingConfirmationCannotRemove(t *testing.T) {
	a, locations := cleanupFixture(t)
	l := locations[0]
	if err := os.Remove(l.Path); err != nil {
		t.Fatal(err)
	}
	m := updateMatches(NewMatches(context.Background(), a, "tiend", catalog.Navigation{Locations: locations}), tea.WindowSizeMsg{Width: 100, Height: 24}, press(tea.KeyEnter))
	b := update(NewBrowser(context.Background(), a, locations[:1], false, "", "", "name", SettingsScreen{}), press(tea.KeyEnter))
	for _, size := range []tea.WindowSizeMsg{{Width: 12, Height: 4}, {Width: 24, Height: 8}, {Width: 100, Height: 4}} {
		m = updateMatches(m, size)
		b = update(b, size)
		for _, key := range []rune{'s', 'y'} {
			model, cmd := m.Update(press(key))
			m = model.(Matches)
			if cmd != nil || m.busy || m.Choice != nil {
				t.Fatal("hidden inline confirmation was accepted")
			}
			model, cmd = b.Update(press(key))
			b = model.(Browser)
			if cmd != nil || b.Busy || b.Choice != nil {
				t.Fatal("hidden Browser confirmation was accepted")
			}
		}
		if m.frameLines() >= size.Height || strings.Count(m.View().Content, "\n")+1 >= size.Height {
			t.Fatal("inline confirmation overflow")
		}
		for _, view := range []string{m.View().Content, b.View().Content} {
			if !strings.Contains(ansi.Strip(view), "Amplía") {
				t.Fatal("hidden confirmation lacks size notice")
			}
			for _, line := range strings.Split(view, "\n") {
				if ansi.StringWidth(line) > size.Width {
					t.Fatal("confirmation width overflow")
				}
			}
		}
	}
	m = updateMatches(m, tea.WindowSizeMsg{Width: 100, Height: 24})
	b = update(b, tea.WindowSizeMsg{Width: 100, Height: 24})
	if !strings.Contains(plainView(m), string(l.ID)) || !strings.Contains(b.View().Content, string(l.ID)) {
		t.Fatal("resize lost target")
	}
	assertCleanupRecords(t, a, l, locations[1], false)
}

func TestBrowserOrdinaryRemovalStillAllowsExistingFolder(t *testing.T) {
	a, locations := cleanupFixture(t)
	l := locations[0]
	b := NewBrowser(context.Background(), a, locations[:1], true, "", "", "name", SettingsScreen{})
	b = update(b, press('d'))
	if b.missing != nil || b.Mode != confirming {
		t.Fatal("manual removal used missing-path policy")
	}
	model, cmd := b.Update(press('s'))
	if cmd == nil {
		t.Fatal("manual removal command missing")
	}
	b = update(model.(Browser), cmd())
	if b.Choice != nil || b.Busy {
		t.Fatal("manual removal navigated")
	}
	if _, err := os.Stat(l.Path); err != nil {
		t.Fatal("manual removal deleted directory", err)
	}
	assertCleanupRecords(t, a, l, locations[1], true)
}

func TestBrowserRefreshCannotOpenMissingConfirmation(t *testing.T) {
	a, locations := cleanupFixture(t)
	if err := os.Remove(locations[0].Path); err != nil {
		t.Fatal(err)
	}
	b := NewBrowser(context.Background(), a, locations[:1], false, "", "", "name", SettingsScreen{})
	b.Busy = true
	b = update(b, press(tea.KeyEnter))
	if b.Mode == confirming || b.Choice != nil {
		t.Fatal("refresh allowed another missing confirmation")
	}
}
