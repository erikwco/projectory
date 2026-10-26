package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/erikwco/projectory/internal/app"
	"github.com/erikwco/projectory/internal/catalog"
)

// matchRows bounds the preview so it stays a short block below the prompt.
const matchRows = 5

// Matches asks which location a typed name means. It renders inline, below the
// shell prompt, instead of taking over the terminal like the browser.
type Matches struct {
	App           *app.App
	Context       context.Context
	Query         string
	Exact         bool
	Locations     []catalog.Location
	Selected      int
	Width, Height int
	Dark, Done    bool
	Notice        string
	Choice        *catalog.Location
	missing       *app.MissingLocation
	direct, busy  bool
	completion    string
	err           error
	// Lines is the tallest preview drawn so far. The preview gets shorter
	// only when the terminal does: Bubble Tea's inline renderer loses its
	// first row when a frame shrinks and leaves the old rows on screen.
	Lines int
}

// NewMatches preselects the closest available location. Locations arrive
// already ordered by closeness. The size stays zero until Bubble Tea reports
// the terminal's, so nothing is drawn or accepted for a guessed size.
func NewMatches(ctx context.Context, a *app.App, query string, n catalog.Navigation) Matches {
	m := Matches{App: a, Context: ctx, Query: query, Exact: n.Exact, Locations: n.Locations, Dark: true}
	for i, l := range m.Locations {
		if l.Available {
			m.Selected = i
			break
		}
	}
	return m
}

func NewMissingMatches(ctx context.Context, a *app.App, missing *app.MissingLocation) Matches {
	m := NewMatches(ctx, a, "", catalog.Navigation{Exact: true, Locations: []catalog.Location{missing.Location()}})
	m.missing, m.direct = missing, true
	return m
}

type missingRemoved struct{ err error }

func (m Matches) Init() tea.Cmd { return tea.RequestBackgroundColor }

// rows is the number of locations the current terminal can show. Zero means
// the preview cannot show the selection, so it must not accept it either.
func (m Matches) rows() int {
	if m.Width < 24 || len(m.Locations) == 0 {
		return 0
	}
	return max(0, min(matchRows, len(m.Locations), m.Height-4))
}

func (m Matches) height() int {
	if m.missing != nil {
		if m.missingFits() {
			return lipgloss.Height(m.missingContent())
		}
		return 1
	}
	if rows := m.rows(); rows > 0 {
		return rows + 3
	}
	return 1
}

func (m Matches) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
		m.Lines = max(m.Lines, m.height())
	case tea.BackgroundColorMsg:
		m.Dark = msg.IsDark()
	case missingRemoved:
		m.busy, m.Done, m.err = false, true, msg.err
		if msg.err == nil {
			m.completion = "Ubicación quitada del catálogo. Sigues en la carpeta actual."
		}
		return m, tea.Quit
	case tea.KeyPressMsg:
		if m.Done || m.busy {
			return m, nil
		}
		if m.missing != nil {
			switch msg.String() {
			case "ctrl+c":
				m.Done = true
				return m, tea.Quit
			case "s", "y":
				if !m.missingFits() {
					return m, nil
				}
				missing, ctx := m.missing, m.Context
				m.busy = true
				return m, func() tea.Msg { return missingRemoved{err: missing.Remove(ctx)} }
			default:
				if m.direct {
					m.Done = true
					return m, tea.Quit
				}
				m.missing = nil
				return m, nil
			}
		}
		last := len(m.Locations) - 1
		switch msg.String() {
		case "esc", "ctrl+c":
			m.Done = true
			return m, tea.Quit
		case "enter":
			if m.rows() == 0 {
				return m, nil
			}
			l := m.Locations[m.Selected]
			if err := m.App.CheckNavigation(l); err != nil {
				if errors.As(err, &m.missing) {
					m.Lines = max(m.Lines, m.height())
				} else {
					m.Notice = err.Error()
				}
				return m, nil
			}
			m.Choice, m.Done = &l, true
			return m, tea.Quit
		case "up", "k", "shift+tab":
			m.Selected = max(0, m.Selected-1)
		case "down", "j", "tab":
			m.Selected = max(0, min(last, m.Selected+1))
		case "home":
			m.Selected = 0
		case "end":
			m.Selected = max(0, last)
		default:
			return m, nil
		}
		m.Notice = ""
	}
	return m, nil
}

// fitPath keeps the end of a path, the part that tells similar folders apart.
func fitPath(path string, width int) string {
	if excess := lipgloss.Width(path) - width; excess > 0 {
		return ansi.TruncateLeft(path, excess+1, "…")
	}
	return path
}

// View keeps the last frame when the person answers; RunMatches erases it
// once the program has stopped drawing.
func (m Matches) View() tea.View {
	if m.Height == 0 {
		return tea.NewView("")
	}
	content := m.content()
	return tea.NewView(content + strings.Repeat("\n", max(0, m.frameLines()-strings.Count(content, "\n")-1)))
}

// frameLines is the height of the padded preview. It stays below the
// terminal's: Bubble Tea redraws a taller frame on every tick, scrolling a
// copy of it into the shell history each time, and a frame as tall as the
// terminal is erased from the top left corner, which tmux also copies into
// history.
func (m Matches) frameLines() int {
	return min(m.Lines, m.Height-1)
}

func (m Matches) content() string {
	p := styles(m.Dark)
	if m.missing != nil {
		if !m.missingFits() {
			return frame("Amplía el terminal · Enter/Esc cancelar", m.Width, 1)
		}
		return m.missingContent()
	}
	rows := m.rows()
	if rows == 0 {
		return frame("Amplía el terminal para elegir · Esc cancelar", m.Width, 1)
	}
	w := min(m.Width, 100)
	title := fmt.Sprintf("Coincidencias para «%s»", Clean(m.Query))
	if m.Exact {
		title = fmt.Sprintf("%s · %d ubicaciones", Clean(m.Locations[0].Project.Name), len(m.Locations))
	}
	position := ""
	if len(m.Locations) > rows {
		position = fmt.Sprintf("%d de %d", m.Selected+1, len(m.Locations))
	}
	lines := []string{columns(p.Title.Render(title), p.Muted.Render(position), w)}
	start := min(max(0, m.Selected-rows+1), len(m.Locations)-rows)
	for i := start; i < start+rows; i++ {
		l := m.Locations[i]
		marker, name := "  ", locationTitle(l)
		if i == m.Selected {
			marker = "> "
		}
		if !m.Exact {
			name = Clean(l.Project.Name) + " · " + name
		}
		right := locationKind(l)
		if l.Branch != "" && l.Branch != l.Label {
			right = Clean(l.Branch)
		}
		if !l.Available {
			right = "no disponible"
		}
		line := columns(marker+name, right, w)
		if i == m.Selected {
			line = p.Selected.Bold(true).Render(line)
		} else if !l.Available {
			line = p.Muted.Render(line)
		}
		lines = append(lines, line)
	}
	status := p.Muted.Render("  " + fitPath(Clean(shorten(m.Locations[m.Selected].Path)), w-2))
	if m.Notice != "" {
		status = p.Error.Render(fit("  "+m.Notice, w))
	}
	help := "↑↓ elegir · Enter abrir · Esc cancelar"
	if w < 40 {
		help = "↑↓ · Enter · Esc"
	}
	lines = append(lines, status, p.Muted.Render(fit(help, w)))
	return strings.Join(lines, "\n")
}

func (m Matches) missingContent() string {
	l := m.missing.Location()
	return wrap(strings.Join([]string{
		"Esta carpeta ya no existe",
		Clean(l.Project.Name) + " · " + locationTitle(l) + " · " + string(l.ID),
		Clean(l.Path),
		"s/y quitar del catálogo · Enter/Esc cancelar",
	}, "\n"), min(m.Width, 100))
}

func (m Matches) missingFits() bool {
	return m.Width >= 24 && lipgloss.Height(m.missingContent()) < m.Height
}

// RunMatches returns no choice after cancellation or cleanup.
func RunMatches(ctx context.Context, m Matches, in io.Reader, out io.Writer) (*catalog.Location, error) {
	model, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return nil, err
	}
	m = model.(Matches)
	if err := m.erase(out); err != nil {
		return nil, err
	}
	if m.completion != "" {
		if _, err := fmt.Fprintln(out, m.completion); err != nil {
			return nil, err
		}
	}
	return m.Choice, m.err
}

// erase removes the final frame once the program has stopped. Bubble Tea
// leaves the cursor at the start of the frame's last row. Going up to its
// first row and erasing below keeps the shell history above it, on the main
// screen.
func (m Matches) erase(out io.Writer) error {
	seq := "\r" + ansi.EraseScreenBelow
	if up := m.frameLines() - 1; up > 0 {
		seq = "\r" + ansi.CursorUp(up) + ansi.EraseScreenBelow
	}
	_, err := io.WriteString(out, seq)
	return err
}
