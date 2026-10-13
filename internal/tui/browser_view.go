package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/erikwco/projectory/internal/catalog"
)

func locationTitle(l catalog.Location) string {
	if l.Label != "" {
		return Clean(l.Label)
	}
	if l.Kind == catalog.Worktree && l.Branch != "" {
		return Clean(l.Branch)
	}
	return Clean(filepath.Base(l.Path))
}

func locationKind(l catalog.Location) string {
	return map[catalog.Kind]string{catalog.Directory: "carpeta", catalog.Repository: "repositorio", catalog.Worktree: "worktree"}[l.Kind]
}

func routeCount(n int) string {
	if n == 1 {
		return "1 ruta"
	}
	return fmt.Sprintf("%d rutas", n)
}

func columns(left, right string, width int) string {
	right = fit(right, max(0, width/2))
	space := max(0, width-lipgloss.Width(right)-2)
	left = fit(left, space)
	return left + strings.Repeat(" ", max(0, width-lipgloss.Width(left)-lipgloss.Width(right))) + right
}

// box accounts for its border and padding inside the requested cell dimensions.
func box(content string, width, height int, p palette) string {
	inner := max(1, width-4)
	lines := strings.Split(frame(content, inner, max(1, height-2)), "\n")
	for len(lines) < height-2 {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lines[i] = p.Border.Render("│") + " " + line + strings.Repeat(" ", max(0, inner-lipgloss.Width(line))) + " " + p.Border.Render("│")
	}
	top := p.Border.Render("╭" + strings.Repeat("─", max(0, width-2)) + "╮")
	bottom := p.Border.Render("╰" + strings.Repeat("─", max(0, width-2)) + "╯")
	return frame(top+"\n"+strings.Join(lines, "\n")+"\n"+bottom, width, height)
}

func wrap(text string, width int) string {
	return lipgloss.NewStyle().Width(max(1, width)).Render(text)
}

func (b Browser) gitLine(l catalog.Location) string {
	if !l.Available {
		return "Carpeta no disponible"
	}
	if l.Repository == nil {
		return ""
	}
	if b.Git == nil || b.Git.ID != l.ID {
		return "Git: consultando…"
	}
	status := b.Git.Status
	if status.Err != nil {
		return "Git: no se pudo leer el estado"
	}
	text := fmt.Sprintf("Git: %d cambios", status.Changed)
	if status.Ahead != "" {
		text += " · " + Clean(status.Ahead) + " / " + Clean(status.Behind) + " local"
	}
	return text
}

func (b Browser) locationDetail(l catalog.Location, width int) string {
	p := styles(b.Dark)
	lines := []string{p.Title.Render(Clean(l.Project.Name)), p.Accent.Render(locationTitle(l)), "", p.Muted.Render("Ruta"), Clean(l.Path), ""}
	metadata := locationKind(l)
	if l.Repository != nil {
		branch := Clean(l.Branch)
		if branch == "" {
			branch = "HEAD separado"
		}
		metadata += " · " + branch
	}
	lines = append(lines, metadata)
	if state := b.gitLine(l); state != "" {
		style := p.Muted
		if !l.Available {
			style = p.Error
		}
		lines = append(lines, style.Render(state))
	}
	if len(l.Technologies) > 0 {
		lines = append(lines, "", p.Muted.Render("Tecnologías"), Clean(strings.Join(l.Technologies, " · ")))
	}
	lines = append(lines, "", fmt.Sprintf("%d visitas · última %s", l.Visits, relative(l.VisitedAt)))
	if l.Repository != nil && l.Repository.Remote != "" {
		lines = append(lines, "", p.Muted.Render("Remoto"), Clean(l.Repository.Remote))
	}
	lines = append(lines, "", p.Muted.Render("ID "+string(l.ID)))
	return wrap(strings.Join(lines, "\n"), width)
}

func (b Browser) projectDetail(width int) string {
	g, ok := b.currentProject()
	if !ok {
		return "Selecciona un proyecto para ver sus datos."
	}
	p := styles(b.Dark)
	lines := []string{p.Title.Render(Clean(g.Project.Name))}
	if g.Project.Description != "" {
		lines = append(lines, "", Clean(g.Project.Description))
	}
	if len(g.Project.Tags) > 0 {
		lines = append(lines, "", p.Accent.Render("#"+Clean(strings.Join(g.Project.Tags, "  #"))))
	}
	lines = append(lines, "", fmt.Sprintf("%s · %d visitas", routeCount(len(g.Locations)), g.Visits), p.Muted.Render("Última visita "+relative(g.VisitedAt)), "")
	for i, l := range g.Locations {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, p.Title.Render(locationTitle(l)))
		metadata := locationKind(l)
		if l.Branch != "" && l.Branch != l.Label {
			metadata += " · " + Clean(l.Branch)
		}
		lines = append(lines, p.Muted.Render(metadata), Clean(shorten(l.Path)))
		if !l.Available {
			lines = append(lines, p.Error.Render("No disponible"))
		}
		if len(l.Technologies) > 0 {
			lines = append(lines, p.Muted.Render(Clean(strings.Join(l.Technologies, " · "))))
		}
		if len(g.Locations) == 1 {
			if state := b.gitLine(l); state != "" && l.Available {
				lines = append(lines, p.Muted.Render(state))
			}
		}
	}
	return wrap(strings.Join(lines, "\n"), width)
}

func (b Browser) projectRows(width, height int) string {
	p := styles(b.Dark)
	if len(b.Visible) == 0 {
		if len(b.Locations) == 0 {
			return frame(p.Title.Render("Tu catálogo empieza aquí")+"\n\n"+"Registra una carpeta con pt add --current.", width, height)
		}
		return frame(p.Title.Render("No hay proyectos que coincidan.")+"\n\n"+"Edita la búsqueda con / o limpia el filtro con Esc.", width, height)
	}
	rowHeight := 2
	if b.Height < 18 {
		rowHeight = 1
	}
	page := max(1, height/rowHeight)
	start := max(0, b.Selected-page+1)
	end := min(len(b.Visible), start+page)
	var lines []string
	for i := start; i < end; i++ {
		g := b.Visible[i]
		marker := "  "
		if i == b.Selected {
			marker = "> "
		}
		right := strings.TrimSpace(strings.Repeat("⚬ ", min(len(g.Locations), 3)))
		if len(g.Locations) > 3 {
			right += " +"
		}
		if !g.available() {
			right = p.Muted.Render("no disponible")
		} else {
			right = p.Accent.Render(right + " ›")
		}
		name := marker + Clean(g.Project.Name)
		if i == b.Selected {
			lines = append(lines, p.Selected.Bold(true).Width(width).Render(columns(name, right, width)))
		} else {
			lines = append(lines, columns(name, right, width))
		}
		if rowHeight > 1 {
			lines = append(lines, p.Rule.Render(strings.Repeat("─", width)))
		}
	}
	return strings.Join(lines, "\n")
}

func (b Browser) mainView() string {
	p := styles(b.Dark)
	w, h := b.Width, b.Height
	title := "PROJECTORY  /  proyectos"
	if b.Dashboard {
		title = "PROJECTORY  /  dashboard"
	}
	order := orderName(b.Order)
	stats := fmt.Sprintf("%d de %d proyectos · %d ubicaciones", len(b.Visible), b.ProjectCount, len(b.Locations))
	if w >= 75 {
		stats += " · orden: " + order
	}
	lines := []string{p.Title.Render(fit(title, w)), p.Muted.Render(fit(stats, w))}
	if b.Dashboard && h >= 22 {
		var visits int64
		missing := 0
		for _, l := range b.Locations {
			visits += l.Visits
			if !l.Available {
				missing++
			}
		}
		lines = append(lines, p.Muted.Render(fmt.Sprintf("%d visitas · %d ubicaciones no disponibles", visits, missing)))
	}
	query := "/  Buscar proyectos"
	if b.Applied != "" {
		query = "/  " + Clean(b.Applied)
	}
	if b.Tag != "" {
		query += "  #" + Clean(b.Tag)
	}
	if b.Mode == searching {
		input := b.Query
		input.SetWidth(w - 4)
		query = input.View()
	}
	lines = append(lines, "", query, "")
	// Short terminals reserve space for the list before secondary chrome.
	if h < 14 {
		lines = []string{p.Title.Render(fit(title, w)), query}
	}
	bodyHeight := max(1, h-len(lines)-2)
	wide := w >= 110 && h >= 18
	listWidth := w
	if wide {
		panelWidth := min(64, w*2/5)
		listWidth = w - panelWidth - 3
		body := lipgloss.NewStyle().Width(listWidth).Height(bodyHeight).Render(b.projectRows(listWidth, bodyHeight))
		content := strings.Split(b.projectDetail(panelWidth-4), "\n")
		contentHeight := bodyHeight - 4
		if len(content) > contentHeight {
			content = content[:contentHeight]
		}
		for len(content) < contentHeight {
			content = append(content, "")
		}
		content = append(content, "", p.Muted.Render("Tab detalle completo"))
		panel := box(strings.Join(content, "\n"), panelWidth, bodyHeight, p)
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, body, "   ", panel))
	} else {
		lines = append(lines, lipgloss.NewStyle().Height(bodyHeight).Render(b.projectRows(listWidth, bodyHeight)))
	}
	position := ""
	if len(b.Visible) > 0 {
		position = fmt.Sprintf("%d/%d", b.Selected+1, len(b.Visible))
	}
	notice := Clean(b.Notice)
	if b.Busy {
		notice = "Actualizando… " + notice
	}
	lines = append(lines, columns(p.Accent.Render(notice), p.Muted.Render(position), w))
	help := "↑↓ elegir  / buscar  Enter abrir  Tab detalle  F2 ajustes  o orden  q salir"
	if b.Dashboard {
		help = "↑↓ elegir  / buscar  Enter abrir  Tab detalle  F2 ajustes  a/e/d administrar  r actualizar  q salir"
	}
	if w < 90 && b.Dashboard {
		help = "↑↓  / buscar  Enter abrir  Tab detalle  F2 ajustes  a/e/d  r  q salir"
	}
	if w < 60 {
		help = "/ buscar · Enter abrir · F2 ajustes · q salir"
	}
	if w < 40 {
		help = "/ · Enter · F2 · q"
	}
	if b.AutoSearch {
		help = "Escribe para filtrar · ↑↓ elegir · Enter abrir · Tab detalle · F2 ajustes · Ctrl+O orden · Ctrl+C salir"
		if b.Dashboard {
			help = "Escribe filtra · ↑↓ · Enter abrir · Tab detalle · F2 ajustes · Ctrl+A/E/D administrar · Ctrl+R actualizar · Ctrl+C salir"
		}
		if w < 120 {
			help = "Escribe filtra · ↑↓ · Enter abrir · F2 ajustes · Ctrl+C salir"
			if b.Dashboard && w >= 90 {
				help = "Escribe filtra · ↑↓ · Enter abrir · F2 ajustes · Ctrl+A/E/D · Ctrl+R · Ctrl+C salir"
			}
		}
		if w < 60 {
			help = "Escribe filtra · ↑↓ · Enter · Ctrl+C"
		}
		if w < 40 {
			help = "↑↓ · Enter · Ctrl+C"
		}
	}
	if b.Mode == searching {
		help = "↑↓ elegir · Enter/Tab ir al listado · Esc cancelar búsqueda · F2 ajustes"
		if w < 72 {
			help = "↑↓ elegir · Enter/Tab lista · Esc · F2"
		}
		if w < 40 {
			help = "↑↓ · Enter/Tab lista · Esc"
		}
	}
	lines = append(lines, p.Muted.Render(fit(help, w)))
	return frame(strings.Join(lines, "\n"), w, h)
}

func (b Browser) pickerView() string {
	g, _ := b.currentProject()
	p := styles(b.Dark)
	w, maxHeight := min(94, b.Width-6), min(b.Height-4, 30)
	if b.Width < 60 || b.Height < 16 {
		w, maxHeight = b.Width, b.Height
	}
	inner := w - 4
	action := map[locationAction]string{openLocation: "Abrir ubicación", editLocation: "Editar ubicación", removeLocation: "Quitar ubicación"}[b.Picker.Action]
	lines := []string{p.Title.Render(fit(Clean(g.Project.Name), inner)), p.Muted.Render(columns(action, routeCount(len(g.Locations)), inner)), ""}
	footer := "↑↓ elegir · Enter confirmar · Tab detalle · Esc volver"
	if w < 64 {
		footer = "↑↓ Enter · Tab detalle · Esc"
	}
	if w < 36 {
		footer = "↑↓ · Enter · Esc"
	}
	rowHeight, detailHeight := 3, 4
	if maxHeight < 20 {
		rowHeight, detailHeight = 2, 2
	}
	if maxHeight < 12 {
		rowHeight, detailHeight = 1, 0
		lines = lines[:2]
	}
	h := min(maxHeight, 2+len(lines)+len(g.Locations)*rowHeight+detailHeight+1)
	listHeight := max(1, h-2-len(lines)-detailHeight-1)
	page := max(1, listHeight/rowHeight)
	start := max(0, b.Picker.Selected-page+1)
	end := min(len(g.Locations), start+page)
	var rows []string
	for i := start; i < end; i++ {
		l := g.Locations[i]
		marker := "  "
		if i == b.Picker.Selected {
			marker = "> "
		}
		right := locationKind(l)
		if l.Branch != "" && l.Branch != l.Label {
			right = Clean(l.Branch)
		}
		if !l.Available {
			right = "no disponible"
		}
		line := columns(marker+locationTitle(l), right, inner)
		if i == b.Picker.Selected {
			line = p.Selected.Bold(true).Render(line)
		}
		rows = append(rows, line)
		if rowHeight >= 2 {
			rows = append(rows, p.Muted.Render(fit("  "+Clean(shorten(l.Path)), inner)))
		}
		if rowHeight >= 3 {
			rows = append(rows, p.Rule.Render(strings.Repeat("─", inner)))
		}
	}
	for len(rows) < listHeight {
		rows = append(rows, "")
	}
	lines = append(lines, rows...)
	if detailHeight > 0 {
		l := g.Locations[b.Picker.Selected]
		summary := wrap(Clean(l.Path), inner)
		if b.Notice != "" {
			summary = p.Error.Render(wrap(Clean(b.Notice), inner))
		} else if state := b.gitLine(l); state != "" {
			summary += "\n" + p.Muted.Render(state)
		}
		detail := strings.Split(frame(summary, inner, detailHeight-1), "\n")
		lines = append(lines, "")
		lines = append(lines, detail...)
		for len(detail) < detailHeight-1 {
			lines = append(lines, "")
			detail = append(detail, "")
		}
	} else if b.Notice != "" {
		footer = Clean(b.Notice)
	}
	lines = append(lines, p.Muted.Render(fit(footer, inner)))
	return p.Dialog.Render(box(strings.Join(lines, "\n"), w, h, p))
}

func (b Browser) confirmationView() string {
	p := styles(b.Dark)
	w := min(86, b.Width-4)
	if b.Width < 50 {
		w = b.Width
	}
	l := b.Removing
	inner := w - 4
	text := []string{p.Title.Render("Quitar del catálogo"), "", Clean(l.Project.Name) + " · " + locationTitle(l), wrap(Clean(l.Path), inner), "", "Se conserva la carpeta y sus archivos.", "", "s quitar · otra tecla cancelar"}
	if b.Height < 14 {
		text = []string{p.Title.Render("Quitar ubicación"), fit(locationTitle(l), inner), fit(Clean(l.Path), inner), "Archivos intactos.", "s quitar · Esc volver"}
	}
	content := strings.Join(text, "\n")
	h := min(b.Height, lipgloss.Height(content)+2)
	return p.Dialog.Render(box(content, w, h, p))
}

func (b Browser) detailView() string {
	p := styles(b.Dark)
	title, content := "Detalle del proyecto", b.projectDetail(b.Width)
	if b.Picker != nil {
		l, _ := b.current()
		title, content = "Detalle de la ubicación", b.locationDetail(l, b.Width)
	}
	rows := strings.Split(content, "\n")
	page := max(1, b.Height-4)
	offset := min(b.DetailOffset, max(0, len(rows)-page))
	lines := []string{p.Title.Render(title), ""}
	lines = append(lines, rows[offset:min(len(rows), offset+page)]...)
	for len(lines) < b.Height-1 {
		lines = append(lines, "")
	}
	help := "↑↓ desplazar · Enter elegir ubicación · Tab/Esc volver"
	if b.Width < 60 {
		help = "↑↓ · Enter elegir · Esc volver"
	}
	lines = append(lines, p.Muted.Render(fit(help, b.Width)))
	return frame(strings.Join(lines, "\n"), b.Width, b.Height)
}

func (b Browser) View() tea.View {
	if b.Width < 24 || b.Height < 8 {
		v := tea.NewView(frame("Amplía el terminal\no usa list --plain\nq salir", b.Width, b.Height))
		v.AltScreen = true
		return v
	}
	if b.Mode == adding || b.Mode == editing {
		f := b.Form
		if b.Busy {
			f.Error = "Guardando…"
		}
		return f.View()
	}
	if b.Mode == settingsMode {
		return b.Settings.View()
	}
	var content string
	if b.Mode == details {
		content = b.detailView()
	} else {
		content = b.mainView()
		var overlay string
		if b.Mode == choosing {
			overlay = b.pickerView()
		} else if b.Mode == confirming {
			overlay = b.confirmationView()
		}
		if overlay != "" {
			x := max(0, (b.Width-lipgloss.Width(overlay))/2)
			y := max(0, (b.Height-lipgloss.Height(overlay))/2)
			content = lipgloss.NewCompositor(lipgloss.NewLayer(content), lipgloss.NewLayer(overlay).X(x).Y(y).Z(1)).Render()
		}
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
