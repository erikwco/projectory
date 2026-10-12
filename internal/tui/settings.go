package tui

import (
	"context"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/erikwco/projectory/internal/catalog"
	"github.com/erikwco/projectory/internal/config"
)

type settingsState uint8

const (
	settingsReady settingsState = iota
	settingsSaving
	settingsClosed
)

type settingsSaved struct {
	Values config.Settings
	Err    error
}

type SettingsScreen struct {
	Values           config.Settings
	Path             string
	Width, Height    int
	Selected         int
	Dark, Standalone bool
	State            settingsState
	Notice, Error    string
}

func NewSettings(path string, values config.Settings) SettingsScreen {
	return SettingsScreen{Path: path, Values: values, Width: 80, Height: 24, Dark: true}
}

func (s SettingsScreen) Init() tea.Cmd { return tea.RequestBackgroundColor }

func (s SettingsScreen) close() (tea.Model, tea.Cmd) {
	s.State = settingsClosed
	if s.Standalone {
		return s, tea.Quit
	}
	return s, nil
}

func (s SettingsScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.Width, s.Height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		s.Dark = msg.IsDark()
	case settingsSaved:
		s.State = settingsReady
		if msg.Err != nil {
			s.Error = msg.Err.Error()
		} else {
			s.Values = msg.Values
			s.Notice = "Guardado"
		}
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return s, tea.Quit
		}
		if s.Width < 24 || s.Height < 8 {
			if msg.String() == "q" {
				return s, tea.Quit
			}
			return s, nil
		}
		if s.State == settingsSaving {
			return s, nil
		}
		switch msg.String() {
		case "esc", "q", "f2":
			return s.close()
		case "up", "down", "tab", "shift+tab":
			s.Selected = (s.Selected + 1) % 2
		case "enter", "space", "left", "right":
			values := s.Values
			if s.Selected == 0 {
				if msg.String() == "left" || msg.String() == "right" {
					return s, nil
				}
				values.SearchOnOpen = !values.SearchOnOpen
			} else {
				values.DefaultSort = catalog.NextOrder(values.SortOrder(), msg.String() == "left")
			}
			s.State = settingsSaving
			s.Notice, s.Error = "", ""
			return s, func() tea.Msg {
				return settingsSaved{Values: values, Err: config.Save(s.Path, values)}
			}
		}
	}
	return s, nil
}

func (s SettingsScreen) View() tea.View {
	if s.Width < 24 || s.Height < 8 {
		v := tea.NewView(frame("Amplía el terminal\npara ver ajustes\nq salir", s.Width, s.Height))
		v.AltScreen = true
		return v
	}
	p := styles(s.Dark)
	w := min(76, s.Width)
	state := "[ ] Desactivado"
	if s.Values.SearchOnOpen {
		state = "[x] Activado"
	}
	if w < 40 {
		state = state[:3]
	}
	row := func(index int, label, value string) []string {
		prefix, style := "  ", p.Title.Bold(false)
		if s.Selected == index {
			prefix, style = "> ", p.Selected.Bold(true)
		}
		if index == 1 && w < 50 {
			return []string{style.Width(w).Render(prefix + label), style.Width(w).Render("  " + value)}
		}
		return []string{style.Width(w).Render(columns(prefix+label, value, w))}
	}
	lines := []string{p.Title.Render("Ajustes"), ""}
	lines = append(lines, row(0, "Buscar al abrir", state)...)
	lines = append(lines, row(1, "Orden predeterminado", orderName(s.Values.SortOrder())+" ›")...)
	if s.Height >= 12 {
		lines = append(lines, "")
		if s.Selected == 0 {
			lines = append(lines,
				wrap("Activado: escribir siempre filtra. ↑↓ mueve la selección; Enter vuelve a la lista.", w), "",
				p.Muted.Render(wrap("Desactivado: pulsa / para buscar.", w)))
		} else {
			description := map[string]string{
				"added":   "Primero los proyectos registrados más recientemente.",
				"visited": "Primero los proyectos que abriste más recientemente.",
				"visits":  "Primero los proyectos que más veces has abierto con pt.",
				"name":    "Los proyectos se muestran por nombre, de A a Z.",
			}[s.Values.SortOrder()]
			lines = append(lines, wrap(description, w))
		}
		lines = append(lines, "", p.Muted.Render(wrap("Se aplica al volver a abrir pt o pt dash.", w)))
	}
	content := frame(strings.Join(lines, "\n"), w, s.Height-2)
	lines = strings.Split(content, "\n")
	for len(lines) < s.Height-2 {
		lines = append(lines, "")
	}
	status := p.Muted.Render("Los cambios se guardan automáticamente.")
	if s.State == settingsSaving {
		status = p.Muted.Render("Guardando…")
	} else if s.Error != "" {
		status = p.Error.Render(Clean(s.Error))
	} else if s.Notice != "" {
		status = p.Accent.Render(s.Notice)
	}
	change := "Espacio / Enter"
	if s.Selected == 1 {
		change = "←→ / Enter"
	}
	help := "↑↓ / Tab elegir · " + change + " cambiar · Esc volver"
	if s.Standalone {
		help = "↑↓ / Tab elegir · " + change + " cambiar · Esc salir"
	}
	if w < 60 {
		help = "↑↓ elegir · Enter cambiar · Esc"
	}
	if w < 40 {
		help = "↑↓ elegir · Enter · Esc"
	}
	lines = append(lines, fit(status, w), p.Muted.Render(fit(help, w)))
	v := tea.NewView(frame(strings.Join(lines, "\n"), s.Width, s.Height))
	v.AltScreen = true
	return v
}

func RunSettings(ctx context.Context, settings SettingsScreen, in io.Reader, out io.Writer) error {
	settings.Standalone = true
	_, err := tea.NewProgram(settings, tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	return err
}
