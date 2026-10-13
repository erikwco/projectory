package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/erikwco/projectory/internal/app"
	"github.com/erikwco/projectory/internal/catalog"
	"github.com/erikwco/projectory/internal/inspect"
)

type mode uint8

const (
	browsing mode = iota
	searching
	confirming
	adding
	editing
	details
	choosing
	settingsMode
)

type loaded struct {
	Locations []catalog.Location
	Notice    string
	Err       error
}
type gitResult struct {
	ID     catalog.LocationID
	Status inspect.Status
}
type inspectSelection catalog.LocationID

type Browser struct {
	App                         *app.App
	Context                     context.Context
	Locations                   []catalog.Location
	Visible                     []projectGroup
	ProjectCount                int
	Picker                      *locationPicker
	Removing                    catalog.Location
	Selected                    int
	DetailOffset                int
	Choice                      *catalog.Location
	Width, Height               int
	Dashboard, Dark, Busy       bool
	AutoSearch                  bool
	Mode                        mode
	Settings                    SettingsScreen
	SettingsReturn              mode
	Query                       textinput.Model
	Applied, Tag, Order, Notice string
	Form                        Form
	Editing                     catalog.Location
	Git                         *gitResult
}

func NewBrowser(ctx context.Context, a *app.App, locations []catalog.Location, dash bool, query, tag, order string, settings SettingsScreen) Browser {
	input := textinput.New()
	input.Prompt = "/ "
	input.Placeholder = "nombre, ruta, rama o tag"
	input.SetVirtualCursor(true)
	input.SetValue(query)
	b := Browser{App: a, Context: ctx, Locations: locations, Dashboard: dash, Query: input, Applied: query, Tag: tag, Order: order, Settings: settings, AutoSearch: settings.Values.SearchOnOpen, Width: 80, Height: 24, Dark: true}
	if settings.Values.SearchOnOpen {
		b.Mode = searching
		b.Query.Focus()
	}
	b.filter()
	return b
}

func (b *Browser) filter() {
	query := b.Applied
	if b.Mode == searching || (b.Mode == settingsMode && b.SettingsReturn == searching) {
		query = b.Query.Value()
	}
	b.Visible, b.ProjectCount = groupProjects(b.Locations, query, b.Tag, b.Order)
	b.Selected = min(b.Selected, max(0, len(b.Visible)-1))
}

func (b Browser) currentProject() (projectGroup, bool) {
	if len(b.Visible) == 0 {
		return projectGroup{}, false
	}
	return b.Visible[b.Selected], true
}

// A project with several locations has no implicit navigation or mutation target.
func (b Browser) current() (catalog.Location, bool) {
	g, ok := b.currentProject()
	if !ok {
		return catalog.Location{}, false
	}
	if b.Picker != nil {
		return g.Locations[b.Picker.Selected], true
	}
	if len(g.Locations) == 1 {
		return g.Locations[0], true
	}
	return catalog.Location{}, false
}

func (b *Browser) returnToList() {
	b.Mode = browsing
	if b.Picker != nil {
		b.Mode = choosing
	}
}

func (b Browser) beginAction(action locationAction) (tea.Model, tea.Cmd) {
	g, ok := b.currentProject()
	if !ok {
		return b, nil
	}
	b.Notice = ""
	if len(g.Locations) > 1 {
		b.Picker = &locationPicker{ProjectID: g.Project.ID, Selected: g.preferredIndex(), Action: action}
		b.Mode = choosing
		b.Git = nil
		return b, b.inspectLater()
	}
	return b.applyAction(action, g.Locations[0])
}

func (b Browser) applyAction(action locationAction, l catalog.Location) (tea.Model, tea.Cmd) {
	if action != openLocation && b.Busy {
		return b, nil
	}
	switch action {
	case openLocation:
		info, err := os.Stat(l.Path)
		if err != nil || !info.IsDir() {
			b.Notice = "Carpeta no disponible. Corrige su ruta desde el dashboard."
			return b, nil
		}
		b.Choice = &l
		return b, tea.Quit
	case editLocation:
		b.Mode = editing
		b.Editing = l
		b.Form = NewForm("Editar ubicación", "Nombre, descripción y tags se comparten con las ubicaciones del proyecto.", []Field{{Label: "Nombre del proyecto", Value: l.Project.Name, Required: true}, {Label: "Descripción", Value: l.Project.Description}, {Label: "Etiqueta de esta ubicación", Value: l.Label}, {Label: "Tags", Value: strings.Join(l.Project.Tags, ", ")}, {Label: "Ruta", Value: l.Path, Required: true}})
		b.Form.Width = b.Width
		b.Form.Height = b.Height
		b.Form.Dark = b.Dark
		return b, b.Form.Init()
	case removeLocation:
		b.Removing = l
		b.Mode = confirming
		return b, nil
	}
	return b, nil
}

func (b Browser) updatePicker(key string) (tea.Model, tea.Cmd) {
	g, _ := b.currentProject()
	switch key {
	case "esc":
		b.Picker = nil
		b.Mode = browsing
		b.Notice = ""
		b.Git = nil
		return b, b.inspectLater()
	case "q":
		return b, tea.Quit
	case "tab":
		b.Mode = details
		b.DetailOffset = 0
		return b, nil
	case "enter":
		return b.applyAction(b.Picker.Action, g.Locations[b.Picker.Selected])
	case "e", "d", "delete":
		if b.Dashboard && !b.Busy {
			action := editLocation
			if key != "e" {
				action = removeLocation
			}
			return b.applyAction(action, g.Locations[b.Picker.Selected])
		}
	case "up", "k":
		b.Picker.Selected = max(0, b.Picker.Selected-1)
	case "down", "j":
		b.Picker.Selected = min(len(g.Locations)-1, b.Picker.Selected+1)
	case "home", "g":
		b.Picker.Selected = 0
	case "end", "G":
		b.Picker.Selected = len(g.Locations) - 1
	case "pgdown":
		b.Picker.Selected = min(len(g.Locations)-1, b.Picker.Selected+max(1, (b.Height-10)/3))
	case "pgup":
		b.Picker.Selected = max(0, b.Picker.Selected-max(1, (b.Height-10)/3))
	default:
		return b, nil
	}
	b.Notice = ""
	b.Git = nil
	return b, b.inspectLater()
}

func (b Browser) inspectLater() tea.Cmd {
	if l, ok := b.current(); ok && l.Repository != nil && l.Available {
		return tea.Tick(140*time.Millisecond, func(time.Time) tea.Msg { return inspectSelection(l.ID) })
	}
	return nil
}

func (b Browser) Init() tea.Cmd {
	var focus tea.Cmd
	if b.Mode == searching {
		focus = b.Query.Focus()
	}
	return tea.Batch(tea.RequestBackgroundColor, b.inspectLater(), focus)
}

func (b Browser) reload(notice string) tea.Msg {
	locations, err := b.App.Store.List(b.Context)
	return loaded{Locations: locations, Notice: notice, Err: err}
}

func (b Browser) mutation() tea.Cmd {
	values := b.Form.Values()
	if b.Mode == adding {
		return func() tea.Msg {
			r, err := b.App.Add(b.Context, app.AddInput{Path: values[0], Name: values[1], Project: values[2], Description: values[3], Tags: catalog.Tags(values[4]), Label: values[5], Discover: true})
			if err != nil {
				return loaded{Err: err}
			}
			return b.reload(fmt.Sprintf("%d ubicaciones agregadas", r.Added))
		}
	}
	location := b.Editing
	location.Project.Name = values[0]
	location.Project.Description = values[1]
	location.Label = values[2]
	location.Project.Tags = catalog.Tags(values[3])
	return func() tea.Msg {
		if err := b.App.Edit(b.Context, location, values[4]); err != nil {
			return loaded{Err: err}
		}
		return b.reload("Cambios guardados")
	}
}

func (b Browser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.Width = msg.Width
		b.Height = msg.Height
		b.Query.SetWidth(max(4, b.Width-4))
		b.Form.Width = msg.Width
		b.Form.Height = msg.Height
		b.Settings.Width, b.Settings.Height = msg.Width, msg.Height
		return b, nil
	case tea.BackgroundColorMsg:
		b.Dark = msg.IsDark()
		b.Form.Dark = b.Dark
		b.Settings.Dark = b.Dark
		b.Query.SetStyles(textinput.DefaultStyles(b.Dark))
		for i := range b.Form.Inputs {
			b.Form.Inputs[i].SetStyles(textinput.DefaultStyles(b.Dark))
		}
		return b, nil
	case loaded:
		b.Busy = false
		if msg.Err != nil {
			b.Notice = msg.Err.Error()
			if b.Mode == adding || b.Mode == editing {
				b.Form.Submitted = false
				b.Form.Error = msg.Err.Error()
			}
			return b, nil
		}
		var keepProject catalog.ProjectID
		var keepLocation catalog.LocationID
		if g, ok := b.currentProject(); ok {
			keepProject = g.Project.ID
		}
		if l, ok := b.current(); ok {
			keepLocation = l.ID
		}
		if b.Mode != searching && b.Mode != details && b.Mode != settingsMode {
			b.Mode = browsing
		}
		b.Locations = msg.Locations
		b.Notice = msg.Notice
		b.filter()
		for i, g := range b.Visible {
			if g.Project.ID == keepProject {
				b.Selected = i
				break
			}
		}
		if b.Picker != nil {
			g, exists := b.currentProject()
			if !exists || g.Project.ID != b.Picker.ProjectID {
				b.Picker = nil
				b.Mode = browsing
			} else {
				b.Picker.Selected = min(b.Picker.Selected, len(g.Locations)-1)
				for i, l := range g.Locations {
					if l.ID == keepLocation {
						b.Picker.Selected = i
						break
					}
				}
				if b.Mode != details {
					b.Mode = choosing
				}
			}
		}
		b.Git = nil
		return b, b.inspectLater()
	case inspectSelection:
		if l, ok := b.current(); ok && l.ID == catalog.LocationID(msg) && l.Repository != nil && l.Available {
			return b, func() tea.Msg { return gitResult{ID: l.ID, Status: inspect.WorkingStatus(b.Context, l.Path)} }
		}
		return b, nil
	case gitResult:
		if l, ok := b.current(); ok && l.ID == msg.ID {
			b.Git = &msg
		}
		return b, nil
	}
	if b.Mode == settingsMode {
		model, cmd := b.Settings.Update(msg)
		b.Settings = model.(SettingsScreen)
		if b.Settings.State == settingsClosed {
			b.Mode = b.SettingsReturn
			if b.Mode == searching {
				return b, b.Query.Focus()
			}
		}
		return b, cmd
	}
	if b.Width < 24 || b.Height < 8 {
		if key, ok := msg.(tea.KeyPressMsg); ok && (key.String() == "q" || key.String() == "ctrl+c") {
			return b, tea.Quit
		}
		return b, nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "f2" && !b.Busy && (b.Mode == browsing || b.Mode == searching) {
		b.SettingsReturn = b.Mode
		b.Mode = settingsMode
		b.Query.Blur()
		b.Settings.State = settingsReady
		b.Settings.Notice, b.Settings.Error = "", ""
		return b, nil
	}
	if b.Mode == adding || b.Mode == editing {
		if b.Busy {
			if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "ctrl+c" {
				return b, tea.Quit
			}
			return b, nil
		}
		form, cmd := b.Form.Update(msg)
		b.Form = form.(Form)
		if b.Form.Cancelled {
			b.returnToList()
			return b, nil
		}
		if b.Form.Submitted {
			b.Busy = true
			return b, b.mutation()
		}
		return b, cmd
	}
	if b.Mode == browsing && b.AutoSearch {
		var resume bool
		switch input := msg.(type) {
		case tea.KeyPressMsg:
			resume = (input.Text != "" && input.Mod&(tea.ModCtrl|tea.ModAlt) == 0 && input.String() != "/") || input.String() == "backspace" || input.String() == "delete"
		case tea.PasteMsg:
			resume = input.Content != ""
		}
		if resume {
			b.Mode = searching
			focus := b.Query.Focus()
			model, cmd := b.Update(msg)
			return model, tea.Batch(focus, cmd)
		}
	}
	if b.Mode == searching {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			switch key.String() {
			case "ctrl+c":
				return b, tea.Quit
			case "enter", "tab", "up", "down":
				b.Applied = b.Query.Value()
				b.Query.Blur()
				b.Mode = browsing
				arrow := key.String() == "up" || key.String() == "down"
				if !arrow {
					b.Selected = 0
				}
				b.filter()
				b.Git = nil
				// Arrows also move the selection; Enter and Tab only focus the list.
				if arrow {
					return b.Update(msg)
				}
				return b, b.inspectLater()
			case "esc":
				b.Query.SetValue(b.Applied)
				b.Query.Blur()
				b.Mode = browsing
				b.filter()
				b.Git = nil
				return b, b.inspectLater()
			}
		}
		previous := b.Query.Value()
		var cmd tea.Cmd
		b.Query, cmd = b.Query.Update(msg)
		if b.Query.Value() != previous {
			b.Selected = 0
			b.filter()
			b.Git = nil
			return b, tea.Batch(cmd, b.inspectLater())
		}
		return b, cmd
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return b, nil
	}
	if key.String() == "ctrl+c" {
		return b, tea.Quit
	}
	if b.Mode == choosing {
		return b.updatePicker(key.String())
	}
	if b.Mode == details {
		switch key.String() {
		case "tab", "esc":
			b.returnToList()
			return b, nil
		case "up", "k":
			b.DetailOffset = max(0, b.DetailOffset-1)
			return b, nil
		case "down", "j":
			b.DetailOffset++
			return b, nil
		case "pgdown":
			b.DetailOffset += max(1, b.Height-4)
			return b, nil
		case "pgup":
			b.DetailOffset = max(0, b.DetailOffset-max(1, b.Height-4))
			return b, nil
		case "enter":
			if b.Picker != nil {
				l, _ := b.current()
				return b.applyAction(b.Picker.Action, l)
			}
			return b.beginAction(openLocation)
		case "q":
		default:
			return b, nil
		}
	}
	if b.Mode == confirming {
		b.returnToList()
		if key.String() == "y" || key.String() == "s" {
			l := b.Removing
			b.Busy = true
			return b, func() tea.Msg {
				if err := b.App.Store.Remove(b.Context, l.ID); err != nil {
					return loaded{Err: err}
				}
				return b.reload("Ubicación quitada del catálogo. Los archivos se conservaron.")
			}
		}
		return b, nil
	}

	action := key.String()
	if b.AutoSearch {
		switch action {
		case "ctrl+a", "ctrl+e", "ctrl+d", "ctrl+r", "ctrl+o":
			action = strings.TrimPrefix(action, "ctrl+")
		}
	}
	switch action {
	case "tab":
		if _, exists := b.currentProject(); exists {
			b.Mode = details
			b.DetailOffset = 0
		}
		return b, nil
	case "q":
		return b, tea.Quit
	case "esc":
		if b.Applied != "" {
			b.Applied = ""
			b.Query.SetValue("")
			b.Selected = 0
			b.filter()
			b.Git = nil
			return b, b.inspectLater()
		}
		return b, tea.Quit
	case "/":
		b.Mode = searching
		b.Query.SetValue(b.Applied)
		return b, b.Query.Focus()
	case "up", "k":
		b.Selected = max(0, b.Selected-1)
		b.Git = nil
		return b, b.inspectLater()
	case "down", "j":
		b.Selected = min(max(0, len(b.Visible)-1), b.Selected+1)
		b.Git = nil
		return b, b.inspectLater()
	case "home", "g":
		b.Selected = 0
		b.Git = nil
		return b, b.inspectLater()
	case "end", "G":
		b.Selected = max(0, len(b.Visible)-1)
		b.Git = nil
		return b, b.inspectLater()
	case "pgdown":
		b.Selected = min(max(0, len(b.Visible)-1), b.Selected+max(1, (b.Height-8)/2))
		b.Git = nil
		return b, b.inspectLater()
	case "pgup":
		b.Selected = max(0, b.Selected-max(1, (b.Height-8)/2))
		b.Git = nil
		return b, b.inspectLater()
	case "enter":
		return b.beginAction(openLocation)
	case "o":
		b.Order = catalog.NextOrder(b.Order, false)
		b.filter()
		b.Git = nil
		return b, b.inspectLater()
	case "r":
		if b.Busy {
			return b, nil
		}
		b.Busy = true
		b.Notice = "Actualizando Git y ubicaciones…"
		return b, func() tea.Msg {
			r, err := b.App.Refresh(b.Context)
			if err != nil {
				return loaded{Err: err}
			}
			notice := fmt.Sprintf("%d revisadas · %d nuevas · %d no disponibles", r.Checked, r.Added, r.Missing)
			if len(r.Warnings) > 0 {
				notice += " · " + r.Warnings[0]
			}
			return b.reload(notice)
		}
	case "d", "delete":
		if b.Dashboard && !b.Busy {
			return b.beginAction(removeLocation)
		}
		return b, nil
	case "a":
		if !b.Dashboard || b.Busy {
			return b, nil
		}
		cwd, _ := os.Getwd()
		b.Mode = adding
		b.Form = NewForm("Registrar ubicación", "Proyecto existente: deja vacío para crear uno o asociar worktrees.", []Field{{Label: "Ruta", Value: cwd, Required: true}, {Label: "Nombre", Hint: "nombre de la carpeta"}, {Label: "Proyecto existente", Hint: "nombre o ID, opcional"}, {Label: "Descripción", Hint: "opcional"}, {Label: "Tags", Hint: "personal, cliente, web"}, {Label: "Etiqueta de esta ubicación", Hint: "principal, copia 2025…"}})
	case "e":
		if b.Dashboard && !b.Busy {
			return b.beginAction(editLocation)
		}
		return b, nil
	default:
		return b, nil
	}
	if b.Mode == adding || b.Mode == editing {
		b.Form.Width = b.Width
		b.Form.Height = b.Height
		b.Form.Dark = b.Dark
		return b, b.Form.Init()
	}
	return b, nil
}

func RunBrowser(ctx context.Context, a *app.App, dash bool, query, tag, order string, settings SettingsScreen, in io.Reader, out io.Writer) (*catalog.Location, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	locations, err := a.Store.List(ctx)
	if err != nil {
		return nil, err
	}
	b := NewBrowser(ctx, a, locations, dash, query, tag, order, settings)
	model, err := tea.NewProgram(b, tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return nil, err
	}
	return model.(Browser).Choice, nil
}
