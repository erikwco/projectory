package tui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
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
	if rows := strings.Count(m.View().Content, "\n") + 1; rows != m.Lines {
		t.Fatalf("the final frame must keep its %d rows for RunMatches to erase: %d", m.Lines, rows)
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
		if cmd == nil || m.Choice != nil || !m.Done {
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
		lines := strings.Split(strings.Trim(plainView(m), "\n"), "\n")
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

// screen replays output the way a terminal without alternate screen shows it,
// with the rows that scroll off the top kept as history. It knows only the
// sequences the preview uses and fails on any other, so a renderer change that
// moves the cursor differently cannot pass unnoticed.
type screen struct {
	t             *testing.T
	width, height int
	history       []string
	rows          [][]rune
	x, y          int
}

func newScreen(t *testing.T, width, height int, history ...string) *screen {
	s := &screen{t: t, width: width, height: height}
	for range height {
		s.rows = append(s.rows, []rune(strings.Repeat(" ", width)))
	}
	for _, line := range history {
		for _, r := range line {
			s.print(r)
		}
		s.newline()
	}
	return s
}

func (s *screen) newline() {
	s.x, s.y = 0, s.y+1
	if s.y == s.height {
		s.history = append(s.history, strings.TrimRight(string(s.rows[0]), " "))
		s.rows = append(s.rows[1:], []rune(strings.Repeat(" ", s.width)))
		s.y--
	}
}

func (s *screen) print(r rune) {
	if s.x == s.width {
		s.newline()
	}
	s.rows[s.y][s.x] = r
	s.x++
}

func (s *screen) clear(row, from int) {
	for x := from; x < s.width; x++ {
		s.rows[row][x] = ' '
	}
}

// resize narrows or widens the screen without reflowing its rows, as xterm.
func (s *screen) resize(width int) {
	for i, row := range s.rows {
		row = append(row, []rune(strings.Repeat(" ", max(0, width-len(row))))...)
		s.rows[i] = row[:width]
	}
	s.width, s.x = width, min(s.x, width-1)
}

func (s *screen) replay(out string) {
	p := ansi.NewParser()
	var state byte
	for len(out) > 0 {
		seq, width, n, next := ansi.DecodeSequence(out, state, p)
		state, out = next, out[n:]
		cmd := ansi.Cmd(p.Command())
		param := func(def int) int { v, _ := p.Param(0, def); return v }
		switch {
		case width > 0:
			s.print([]rune(seq)[0])
		case seq == "\r":
			s.x = 0
		case seq == "\n": // The renderer writes to a pipe here, which it treats as translating LF to CRLF.
			s.newline()
		case strings.HasPrefix(seq, "\x1b]9999;"): // The test's own marker for a resize.
			w, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b]9999;"), "\a"))
			s.resize(w)
		case ansi.HasOscPrefix(seq), ansi.HasCsiPrefix(seq) && (cmd.Prefix() != 0 || cmd.Intermediate() != 0 || cmd.Final() == 'm'):
			// Colors, styles, modes, and queries leave the cells and cursor alone.
		case ansi.HasCsiPrefix(seq) && cmd.Final() == 'A':
			s.x, s.y = min(s.x, s.width-1), max(0, s.y-param(1))
		case ansi.HasCsiPrefix(seq) && cmd.Final() == 'B':
			s.x, s.y = min(s.x, s.width-1), min(s.height-1, s.y+param(1))
		case ansi.HasCsiPrefix(seq) && cmd.Final() == 'C':
			s.x = min(s.width-1, s.x+param(1))
		case ansi.HasCsiPrefix(seq) && cmd.Final() == 'D':
			s.x = max(0, min(s.x, s.width-1)-param(1))
		case ansi.HasCsiPrefix(seq) && cmd.Final() == 'G':
			s.x = min(s.width-1, max(1, param(1))-1)
		case ansi.HasCsiPrefix(seq) && cmd.Final() == 'K' && param(0) == 0:
			s.clear(s.y, min(s.x, s.width))
		case ansi.HasCsiPrefix(seq) && cmd.Final() == 'J' && param(0) == 0:
			s.clear(s.y, min(s.x, s.width))
			for row := s.y + 1; row < s.height; row++ {
				s.clear(row, 0)
			}
		default:
			s.t.Fatalf("the screen does not model %q", seq)
		}
	}
}

// text returns the history and the screen, without trailing blank rows, and
// the cursor's row counted from the first history row.
func (s *screen) text() ([]string, int) {
	lines := slices.Clone(s.history)
	for _, row := range s.rows {
		lines = append(lines, strings.TrimRight(string(row), " "))
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, len(s.history) + s.y
}

// The renderer and RunMatches' erase together must give back the screen as it
// was, after scrolling, moving, resizing, cancelling, or choosing.
func TestMatchesProgramLeavesNoRowsBehind(t *testing.T) {
	history := []string{"$ ls", "notas.txt", "$ cd ~/dev", "$ pt tien"}
	cases := []struct {
		name   string
		events []any
		choice catalog.LocationID
	}{
		{"cancel", []any{press(tea.KeyDown), press(tea.KeyEscape)}, ""},
		{"ctrl+c", []any{tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}}, ""},
		{"choose", []any{press(tea.KeyDown), press(tea.KeyDown), press(tea.KeyEnter)}, "lC"},
		{"narrow and widen", []any{20, 80, press(tea.KeyDown), press(tea.KeyEscape)}, ""},
		{"stay narrow", []any{press(tea.KeyDown), 20, press(tea.KeyEscape)}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := &lockedBuffer{}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			program := tea.NewProgram(NewMatches("tien", sampleMatches(8)), tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(out), tea.WithWindowSize(80, 10))
			done := make(chan Matches, 1)
			go func() {
				model, err := program.Run()
				if err != nil {
					t.Error(err)
				}
				final, _ := model.(Matches)
				done <- final
			}()
			wait := func(what string, ready func() bool) {
				for !ready() {
					select {
					case <-ctx.Done():
						t.Fatalf("%s: %q", what, out.String())
					case <-time.After(10 * time.Millisecond):
					}
				}
			}
			wait("the preview never rendered", func() bool { return strings.Contains(out.String(), "Coincidencias") })
			// Each event waits for its frame, as a person would. An int is a
			// new terminal width; the marker tells the screen when it applies.
			for _, event := range c.events {
				switch event := event.(type) {
				case int:
					_, _ = fmt.Fprintf(out, "\x1b]9999;%d\a", event)
					mark := len(out.String())
					program.Send(tea.WindowSizeMsg{Width: event, Height: 10})
					wait("the resize drew nothing", func() bool { return len(out.String()) > mark })
				case tea.KeyPressMsg:
					mark := len(out.String())
					program.Send(event)
					wait(event.String()+" drew nothing", func() bool { return len(out.String()) > mark })
				}
				time.Sleep(50 * time.Millisecond)
			}
			final := <-done
			if err := final.erase(out); err != nil {
				t.Fatal(err)
			}
			if final.Choice == nil && c.choice != "" || final.Choice != nil && final.Choice.ID != c.choice {
				t.Fatalf("wrong choice: %+v", final.Choice)
			}
			term := newScreen(t, 80, 10, history...)
			term.replay(out.String())
			lines, row := term.text()
			if !slices.Equal(lines, history) || row != len(history) || term.x != 0 {
				t.Fatalf("the preview left rows or moved the prompt (cursor row %d, column %d):\n%s", row, term.x, strings.Join(lines, "\n"))
			}
		})
	}
}
