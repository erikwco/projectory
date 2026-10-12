package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (f Form) View() tea.View {
	view := tea.NewView("")
	view.AltScreen = true
	if f.Width < 24 || f.Height < 8 {
		view.Content = frame("Amplía el terminal\npara ver formulario\nq salir", f.Width, f.Height)
		return view
	}

	p := styles(f.Dark)
	framed := f.Width >= 60 && f.Height >= 18
	width, height, top := min(76, f.Width), f.Height, 0
	if framed {
		width = min(80, f.Width-4) - 4
		top = 1
		if f.Height >= 28 {
			top = 2
		}
		height -= top + 3 // Top margin, border, and one row below the panel.
	}

	statusRows := 1
	if height >= 16 {
		statusRows = 2
	}
	footerRows := statusRows + 2 // Divider and actions stay in place while fields scroll.
	noteRows := min(4, max(0, height-footerRows-4))
	lines := []string{p.Title.Render(fit(Clean(f.Title), width))}
	if f.Note != "" && noteRows > 0 {
		for _, line := range formTextLines(f.Note, width, noteRows) {
			lines = append(lines, p.Muted.Render(line))
		}
	}
	if height-len(lines)-footerRows >= 3 {
		lines = append(lines, "")
	}

	bodyRows := height - len(lines) - footerRows
	gap := 0
	if 3*len(f.Fields)-1 <= bodyRows {
		gap = 1
	}
	visible := min(len(f.Fields), max(1, (bodyRows+gap)/(2+gap)))
	start := max(0, f.Focus-visible+1)
	end := min(len(f.Fields), start+visible)
	for i := start; i < end; i++ {
		label := Clean(f.Fields[i].Label)
		if f.Fields[i].Required {
			label += " *"
		}
		style := p.Muted
		if i == f.Focus {
			style = p.Accent.Bold(true)
		}
		if i == f.InvalidField {
			style = p.Error.Bold(true)
		}
		lines = append(lines, style.Render(fit(label, width)), f.inputView(i, width, p))
		if gap > 0 && i < end-1 {
			lines = append(lines, "")
		}
	}
	for len(lines) < height-footerRows {
		lines = append(lines, "")
	}
	lines = append(lines, p.Rule.Render(strings.Repeat("─", width)))
	lines = append(lines, f.statusView(start, end, width, statusRows, p)...)
	actions := p.Accent.Bold(true).Render("Ctrl+S guardar") + p.Muted.Render("   Esc cancelar")
	if width < 36 {
		actions = p.Accent.Bold(true).Render("Ctrl+S guardar") + p.Muted.Render(" · Esc")
	}
	if f.Submitted && !f.Standalone {
		actions = p.Muted.Render("Ctrl+C salir")
	}
	lines = append(lines, actions)

	content := strings.Join(lines, "\n")
	if framed {
		content = box(content, width+4, height+2, p)
		width += 4
	}
	indent := strings.Repeat(" ", max(0, (f.Width-width)/2))
	content = indent + strings.ReplaceAll(content, "\n", "\n"+indent)
	view.Content = frame(strings.Repeat("\n", top)+content, f.Width, f.Height)
	return view
}

func (f Form) inputView(index, width int, p palette) string {
	input := f.Inputs[index]
	active := index == f.Focus
	input.Prompt = "  "
	style := textinput.DefaultStyles(f.Dark)
	style.Blurred.Text = lipgloss.NewStyle()
	style.Blurred.Placeholder = p.Muted
	style.Focused.Text = p.Selected
	style.Focused.Placeholder = p.Accent.Background(p.Selected.GetBackground())
	style.Focused.Prompt = p.Accent.Background(p.Selected.GetBackground())
	style.Cursor.Color = p.Accent.GetForeground()
	if active {
		input.Prompt = "> "
	}
	if f.Submitted {
		input.Blur()
	}
	input.SetStyles(style)
	input.SetWidth(max(1, width-3)) // Prompt plus a cell for the cursor at the end.
	// Bubbles caches its horizontal viewport. Changing width alone does not
	// recompute it, so reflow on a copy and restore the actual cursor position.
	position := input.Position()
	input.CursorEnd()
	input.SetCursor(position)
	value := fit(input.View(), width)
	if active {
		return p.Selected.Width(width).Render(value)
	}
	return value
}

func (f Form) statusView(start, end, width, rows int, p palette) []string {
	if f.Error != "" || (f.Submitted && !f.Standalone) {
		message, style := f.Error, p.Error
		if f.Submitted && !f.Standalone {
			message, style = "Guardando…", p.Muted
		}
		lines := formTextLines(message, width, rows)
		for len(lines) < rows {
			lines = append(lines, "")
		}
		for i := range lines {
			lines[i] = style.Render(lines[i])
		}
		return lines
	}
	progress := fmt.Sprintf("Campo %d de %d", f.Focus+1, len(f.Fields))
	remaining := "* obligatorio"
	if start > 0 || end < len(f.Fields) {
		remaining = fmt.Sprintf("↑ %d  ↓ %d", start, len(f.Fields)-end)
	}
	lines := []string{p.Muted.Render(columns(progress, remaining, width))}
	if rows > 1 {
		lines = append(lines, p.Muted.Render("Tab / Shift+Tab cambiar de campo"))
	}
	return lines
}

func formTextLines(text string, width, limit int) []string {
	lines := strings.Split(wrap(Clean(text), width), "\n")
	if len(lines) > limit {
		lines = lines[:limit]
		lines[limit-1] = fit(strings.TrimRight(lines[limit-1], " ")+" …", width)
	}
	return lines
}
