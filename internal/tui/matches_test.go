package tui

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/erikwco/projectory/internal/catalog"
)

func sampleMatches(count int) catalog.Navigation {
	var locations []catalog.Location
	for i := range count {
		name := string(rune('A' + i))
		locations = append(locations, catalog.Location{ID: catalog.LocationID("l" + name), Project: catalog.Project{ID: catalog.ProjectID("p" + name), Name: "Tienda " + name}, Path: "/dev/tienda-" + name, Label: "principal", Kind: catalog.Directory, Available: true})
	}
	return catalog.Navigation{Locations: locations}
}

func updateMatches(m Matches, msgs ...tea.Msg) Matches {
	for _, msg := range msgs {
		model, _ := m.Update(msg)
		m = model.(Matches)
	}
	return m
}

func plainView(m Matches) string { return ansi.Strip(m.View().Content) }

func TestMatchesEnterChoosesPreselectedAvailableLocation(t *testing.T) {
	found := sampleMatches(3)
	found.Locations[0].Available = false
	m := NewMatches("tien", found)
	if m.Selected != 1 {
		t.Fatalf("closest available location was not preselected: %d", m.Selected)
	}
	m = updateMatches(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	model, cmd := m.Update(press(tea.KeyEnter))
	m = model.(Matches)
	if cmd == nil || m.Choice == nil || m.Choice.ID != "lB" || !m.Done {
		t.Fatalf("Enter did not choose the selection: %+v", m.Choice)
	}
	if m.View().Content != "" {
		t.Fatal("the preview must disappear after choosing")
	}
}

func TestMatchesRefusesUnavailableLocation(t *testing.T) {
	found := sampleMatches(2)
	found.Locations[1].Available = false
	m := updateMatches(NewMatches("tien", found), press(tea.KeyDown))
	if !strings.Contains(plainView(m), "no disponible") {
		t.Fatal("unavailable location is not marked")
	}
	model, cmd := m.Update(press(tea.KeyEnter))
	m = model.(Matches)
	if cmd != nil || m.Choice != nil || m.Done || !strings.Contains(plainView(m), "Carpeta no disponible") {
		t.Fatal("an unavailable folder was accepted")
	}
	m = updateMatches(m, press(tea.KeyUp))
	if m.Notice != "" {
		t.Fatal("moving did not clear the notice")
	}
}

func TestMatchesCancelWithEscOrCtrlC(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{press(tea.KeyEscape), {Code: 'c', Mod: tea.ModCtrl}} {
		model, cmd := NewMatches("tien", sampleMatches(2)).Update(key)
		m := model.(Matches)
		if cmd == nil || m.Choice != nil || !m.Done || m.View().Content != "" {
			t.Fatalf("%s did not cancel cleanly", key)
		}
	}
}

func TestMatchesScrollsWithoutDroppingCandidates(t *testing.T) {
	m := updateMatches(NewMatches("tien", sampleMatches(8)), tea.WindowSizeMsg{Width: 80, Height: 24})
	view := plainView(m)
	if strings.Count(view, "Tienda ") != matchRows || !strings.Contains(view, "1 de 8") {
		t.Fatalf("expected a five-row window with a position:\n%s", view)
	}
	for range 10 {
		m = updateMatches(m, press(tea.KeyDown))
	}
	view = plainView(m)
	if m.Selected != 7 || !strings.Contains(view, "> Tienda H") || !strings.Contains(view, "8 de 8") || strings.Contains(view, "Tienda A") {
		t.Fatalf("scrolling lost the selection:\n%s", view)
	}
	if !strings.Contains(view, "/dev/tienda-H") {
		t.Fatal("selected path is not visible")
	}
	m = updateMatches(m, press(tea.KeyHome))
	if m.Selected != 0 || !strings.Contains(plainView(m), "> Tienda A") {
		t.Fatal("Home did not return to the closest match")
	}
}

func TestMatchesAdaptsToResizeAndBlocksHiddenChoice(t *testing.T) {
	m := NewMatches("tien", sampleMatches(8))
	for _, size := range []tea.WindowSizeMsg{{Width: 30, Height: 6}, {Width: 120, Height: 40}} {
		m = updateMatches(m, size)
		lines := strings.Split(plainView(m), "\n")
		if len(lines) > size.Height-1 {
			t.Fatalf("preview taller than the terminal at %v: %d lines", size, len(lines))
		}
		for _, line := range lines {
			if width := ansi.StringWidth(line); width > size.Width {
				t.Fatalf("line wider than %d: %q", size.Width, line)
			}
		}
		if !strings.Contains(plainView(m), "> Tienda A") {
			t.Fatalf("selection hidden at %v", size)
		}
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 20, Height: 24}, {Width: 80, Height: 4}} {
		m = updateMatches(m, size)
		model, cmd := m.Update(press(tea.KeyEnter))
		m = model.(Matches)
		if cmd != nil || m.Choice != nil || !strings.Contains(plainView(m), "Amplía el terminal") {
			t.Fatalf("Enter accepted a selection the terminal does not show at %v", size)
		}
	}
	m = updateMatches(m, tea.WindowSizeMsg{Width: 80, Height: 24}, press(tea.KeyEnter))
	if m.Choice == nil {
		t.Fatal("selection did not recover after enlarging the terminal")
	}
}

func TestMatchesCleansControlCharactersAndShortensLongPaths(t *testing.T) {
	found := sampleMatches(1)
	found.Locations[0].Project.Name = "Evil\x1b[31m\nName"
	found.Locations[0].Branch = "feature/\x07bell"
	found.Locations[0].Path = "/very/long/" + strings.Repeat("nested/", 20) + "final-folder"
	m := updateMatches(NewMatches("ev\x1bil", found), tea.WindowSizeMsg{Width: 50, Height: 10})
	content := m.View().Content
	if strings.ContainsAny(ansi.Strip(content), "\x1b\x07") || strings.Contains(content, "[31m") {
		t.Fatalf("control characters reached the terminal: %q", content)
	}
	if !strings.Contains(ansi.Strip(content), "…") || !strings.Contains(ansi.Strip(content), "final-folder") {
		t.Fatalf("long path should keep its end:\n%s", ansi.Strip(content))
	}
}

func TestMatchesExactTitleNamesProject(t *testing.T) {
	found := sampleMatches(2)
	found.Exact = true
	found.Locations[1].Project = found.Locations[0].Project
	found.Locations[1].Label = "copia"
	view := plainView(NewMatches("Tienda A", found))
	if !strings.Contains(view, "Tienda A · 2 ubicaciones") || !strings.Contains(view, "> principal") || !strings.Contains(view, "copia") {
		t.Fatalf("exact name with several locations:\n%s", view)
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// The program runs as RunMatches does, with the size a terminal would report.
func TestMatchesProgramRendersInlineWithoutAlternateScreen(t *testing.T) {
	if NewMatches("tien", sampleMatches(2)).View().AltScreen {
		t.Fatal("the preview must stay in the shell's screen")
	}
	in, keys := io.Pipe()
	out := &lockedBuffer{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan *catalog.Location, 1)
	go func() {
		model, err := tea.NewProgram(NewMatches("tien", sampleMatches(2)), tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out), tea.WithWindowSize(80, 24)).Run()
		if err != nil {
			t.Error(err)
			done <- nil
			return
		}
		done <- model.(Matches).Choice
	}()
	for !strings.Contains(out.String(), "Coincidencias") {
		select {
		case <-ctx.Done():
			t.Fatalf("preview never rendered: %q", out.String())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if _, err := keys.Write([]byte("\x1b[B\r")); err != nil {
		t.Fatal(err)
	}
	choice := <-done
	if choice == nil || choice.ID != "lB" {
		t.Fatalf("wrong choice: %+v", choice)
	}
	if strings.Contains(out.String(), "\x1b[?1049h") {
		t.Fatal("preview switched to the alternate screen")
	}
}
