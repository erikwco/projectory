package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type palette struct{ Title, Muted, Selected, Accent, Error, Rule, Border, Dialog lipgloss.Style }

func styles(dark bool) palette {
	color := lipgloss.LightDark(dark)
	return palette{
		Title:    lipgloss.NewStyle().Bold(true),
		Muted:    lipgloss.NewStyle().Foreground(color(lipgloss.Color("#596575"), lipgloss.Color("#96A4B6"))),
		Selected: lipgloss.NewStyle().Foreground(color(lipgloss.Color("#093F48"), lipgloss.Color("#D8FBFF"))).Background(color(lipgloss.Color("#D7F2F4"), lipgloss.Color("#183840"))),
		Accent:   lipgloss.NewStyle().Foreground(color(lipgloss.Color("#086B76"), lipgloss.Color("#79DCE4"))),
		Error:    lipgloss.NewStyle().Foreground(color(lipgloss.Color("#A82727"), lipgloss.Color("#FFA79E"))),
		Rule:     lipgloss.NewStyle().Foreground(color(lipgloss.Color("#D2D8DF"), lipgloss.Color("#364453"))),
		Border:   lipgloss.NewStyle().Foreground(color(lipgloss.Color("#8895A5"), lipgloss.Color("#536579"))),
		Dialog:   lipgloss.NewStyle().Foreground(color(lipgloss.Color("#202C3B"), lipgloss.Color("#E3EAF3"))).Background(color(lipgloss.Color("#F5F7FA"), lipgloss.Color("#1D2733"))),
	}
}

// Clean makes untrusted filesystem and Git text safe to render in a terminal.
func Clean(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, ansi.Strip(text))
}

func shorten(path string) string {
	home, err := os.UserHomeDir()
	if err == nil {
		if path == home {
			return "~"
		}
		if strings.HasPrefix(path, home+string(filepath.Separator)) {
			return "~" + strings.TrimPrefix(path, home)
		}
	}
	return path
}

func fit(text string, width int) string { return ansi.Truncate(text, max(0, width), "…") }

func orderName(order string) string {
	return map[string]string{"added": "registro", "visited": "última visita", "visits": "visitas", "name": "nombre"}[order]
}

func frame(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = fit(line, width)
	}
	return strings.Join(lines, "\n")
}

func relative(t *time.Time) string {
	if t == nil {
		return "nunca"
	}
	d := time.Since(*t)
	if d < time.Minute {
		return "ahora"
	}
	if d < time.Hour {
		return fmt.Sprintf("hace %d min", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("hace %d h", int(d.Hours()))
	}
	return fmt.Sprintf("hace %d días", int(d.Hours()/24))
}
