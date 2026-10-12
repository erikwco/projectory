package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func updateForm(f Form, msg tea.Msg) Form {
	model, _ := f.Update(msg)
	return model.(Form)
}

func TestFormLongInputKeepsCursorVisibleAcrossResize(t *testing.T) {
	path := "START/" + strings.Repeat("carpeta/日本語/", 12) + "END_MARK"
	f := NewForm("Editar ubicación", "", []Field{{Label: "Ruta", Value: path, Required: true}})
	f.Init()
	for _, size := range [][2]int{{120, 40}, {40, 12}, {24, 8}, {80, 24}} {
		f = updateForm(f, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		f = updateForm(f, tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
		if !strings.Contains(ansi.Strip(f.View().Content), "END_MARK") {
			t.Fatalf("cursor's end of the path is hidden at %v", size)
		}
		f = updateForm(f, tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
		if !strings.Contains(ansi.Strip(f.View().Content), "START/") {
			t.Fatalf("cursor's start of the path is hidden at %v", size)
		}
		f = updateForm(f, press(tea.KeyRight))
		f = updateForm(f, tea.KeyPressMsg{Code: 'X', Text: "X"})
		if f.Values()[0] != "SX"+path[1:] {
			t.Fatal("rendering or resizing changed the value or cursor position")
		}
		f = updateForm(f, press(tea.KeyBackspace))
	}
}

func TestStandaloneFormBlocksHiddenInputWhenUndersized(t *testing.T) {
	f := NewForm("Registrar proyecto", "", []Field{{Label: "Nombre", Value: "Demo", Required: true}})
	f.Standalone = true
	f.Init()
	f = updateForm(f, tea.WindowSizeMsg{Width: 12, Height: 4})
	for _, msg := range []tea.Msg{press(tea.KeyEnter), press(tea.KeyTab), tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}, tea.KeyPressMsg{Code: 'x', Text: "x"}, tea.PasteMsg{Content: "hidden"}} {
		model, cmd := f.Update(msg)
		after := model.(Form)
		if cmd != nil || after.Submitted || after.Focus != 0 || after.Values()[0] != "Demo" {
			t.Fatal("an invisible form accepted input or submitted")
		}
	}
	if !strings.Contains(f.View().Content, "q salir") {
		t.Fatal("tiny form should advertise how to exit")
	}
	model, cmd := f.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil || !model.(Form).Cancelled {
		t.Fatal("tiny standalone form cannot be cancelled")
	}
}

func TestFormKeepsFocusedFieldAndActionsVisible(t *testing.T) {
	fields := []Field{
		{Label: "Ruta", Value: strings.Repeat("path/", 30), Required: true},
		{Label: "Nombre"}, {Label: "Proyecto"}, {Label: "Descripción"}, {Label: "Tags"}, {Label: "Etiqueta"},
	}
	for _, dark := range []bool{true, false} {
		for _, size := range [][2]int{{120, 40}, {80, 24}, {48, 20}, {40, 12}, {24, 8}} {
			f := NewForm("Registrar ubicación", "Proyecto existente: deja vacío para crear uno o asociar worktrees.", fields)
			f.Dark = dark
			f.Init()
			f = updateForm(f, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			footerRow := -1
			for range fields {
				view := ansi.Strip(f.View().Content)
				if !strings.Contains(view, f.Fields[f.Focus].Label) || !strings.Contains(view, "Ctrl+S") || !strings.Contains(view, "Esc") {
					t.Fatalf("focused field or actions missing at %v, focus %d: %q", size, f.Focus, view)
				}
				if lipgloss.Height(view) > size[1] {
					t.Fatalf("form height overflow at %v", size)
				}
				for row, line := range strings.Split(view, "\n") {
					if lipgloss.Width(line) > size[0] {
						t.Fatalf("form width overflow at %v", size)
					}
					if strings.Contains(line, "Ctrl+S") {
						if footerRow >= 0 && footerRow != row {
							t.Fatal("actions moved when focus changed")
						}
						footerRow = row
					}
				}
				f = updateForm(f, press(tea.KeyTab))
			}
		}
	}
}

func TestFormValidationFocusesMissingFieldAndRecovers(t *testing.T) {
	f := NewForm("Registrar ubicación", "", []Field{{Label: "Ruta", Required: true}, {Label: "Nombre", Value: "Demo"}})
	f.Init()
	f = updateForm(f, press(tea.KeyTab))
	f = updateForm(f, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if f.Submitted || f.Focus != 0 || f.InvalidField != 0 || f.Error != "Completa ruta" {
		t.Fatalf("missing required field was not exposed: %+v", f)
	}
	before := strings.Split(ansi.Strip(f.View().Content), "\n")
	f = updateForm(f, tea.KeyPressMsg{Code: ' ', Text: " "})
	if f.Error == "" {
		t.Fatal("whitespace dismissed a required-field error")
	}
	f = updateForm(f, tea.PasteMsg{Content: "/tmp/Demo"})
	if f.Error != "" || f.InvalidField != -1 {
		t.Fatal("correcting a required field left its error visible")
	}
	f = updateForm(f, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if !f.Submitted || !strings.Contains(f.View().Content, "Guardando") {
		t.Fatal("valid form should show saving while the browser persists it")
	}
	after := strings.Split(ansi.Strip(f.View().Content), "\n")
	for row, line := range before {
		if strings.Contains(line, "Ctrl+S") && !strings.Contains(after[row], "Ctrl+C") {
			t.Fatal("the action row moved between validation and saving")
		}
	}
}
