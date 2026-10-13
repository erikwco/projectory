package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/erikwco/projectory/internal/catalog"
)

// matchRows bounds the preview so it stays a short block below the prompt.
const matchRows = 5

// Matches asks which location a typed name means. It renders inline, below the
// shell prompt, instead of taking over the terminal like the browser.
type Matches struct {
	Query         string
	Exact         bool
	Locations     []catalog.Location
	Selected      int
	Width, Height int
	Dark, Done    bool
	Notice        string
	Choice        *catalog.Location
}

// NewMatches preselects the closest available location. Locations arrive
// already ordered by closeness.
func NewMatches(query string, n catalog.Navigation) Matches {
	m := Matches{Query: query, Exact: n.Exact, Locations: n.Locations, Width: 80, Height: 24, Dark: true}
	for i, l := range m.Locations {
		if l.Available {
			m.Selected = i
			break
		}
	}
	return m
}

func (m Matches) Init() tea.Cmd { return tea.RequestBackgroundColor }

// rows is the number of locations the current terminal can show. Zero means
// the preview cannot show the selection, so it must not accept it either.
func (m Matches) rows() int {
	if m.Width < 24 || len(m.Locations) == 0 {
		return 0
	}
	return max(0, min(matchRows, len(m.Locations), m.Height-4))
}

func (m Matches) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		m.Dark = msg.IsDark()
	case tea.KeyPressMsg:
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
			if !l.Available {
				m.Notice = "Carpeta no disponible; elige otra ubicación."
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

func (m Matches) View() tea.View {
	// An empty final frame erases the preview, leaving the prompt as it was.
	if m.Done {
		return tea.NewView("")
	}
	p := styles(m.Dark)
	rows := m.rows()
	if rows == 0 {
		return tea.NewView(frame("Amplía el terminal para elegir · Esc cancelar", m.Width, max(1, m.Height-1)))
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
	return tea.NewView(strings.Join(lines, "\n"))
}

// RunMatches returns nil when the person cancels.
func RunMatches(ctx context.Context, m Matches, in io.Reader, out io.Writer) (*catalog.Location, error) {
	model, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return nil, err
	}
	return model.(Matches).Choice, nil
}
