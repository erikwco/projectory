package tui

import (
	"context"
	"io"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type Field struct {
	Label, Value, Hint string
	Required           bool
}

type Form struct {
	Title, Note, Error                     string
	Fields                                 []Field
	Inputs                                 []textinput.Model
	Focus, Width, Height                   int
	InvalidField                           int
	Submitted, Cancelled, Standalone, Dark bool
}

func NewForm(title, note string, fields []Field) Form {
	f := Form{Title: title, Note: note, Fields: fields, Width: 80, Height: 24, Dark: true, InvalidField: -1}
	for _, field := range fields {
		input := textinput.New()
		input.Prompt = "> "
		input.CharLimit = 4096
		input.SetValue(field.Value)
		input.Placeholder = field.Hint
		input.SetVirtualCursor(true)
		f.Inputs = append(f.Inputs, input)
	}
	return f
}

func (f Form) Init() tea.Cmd { return tea.Batch(f.Inputs[0].Focus(), tea.RequestBackgroundColor) }

func (f Form) Values() []string {
	values := make([]string, len(f.Inputs))
	for i, input := range f.Inputs {
		values[i] = input.Value()
	}
	return values
}

func (f Form) submit() (tea.Model, tea.Cmd) {
	for i, field := range f.Fields {
		if field.Required && strings.TrimSpace(f.Inputs[i].Value()) == "" {
			f.Error = "Completa " + strings.ToLower(field.Label)
			f.InvalidField = i
			f.Inputs[f.Focus].Blur()
			f.Focus = i
			return f, f.Inputs[i].Focus()
		}
	}
	f.Error = ""
	f.InvalidField = -1
	f.Submitted = true
	if f.Standalone {
		return f, tea.Quit
	}
	return f, nil
}

func (f Form) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if f.Width < 24 || f.Height < 8 {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			if msg.String() == "q" || msg.String() == "ctrl+c" || msg.String() == "esc" {
				f.Cancelled = true
				if f.Standalone {
					return f, tea.Quit
				}
			}
			return f, nil
		case tea.PasteMsg:
			return f, nil
		}
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		f.Width = msg.Width
		f.Height = msg.Height
	case tea.BackgroundColorMsg:
		f.Dark = msg.IsDark()
		for i := range f.Inputs {
			f.Inputs[i].SetStyles(textinput.DefaultStyles(f.Dark))
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			f.Cancelled = true
			if f.Standalone {
				return f, tea.Quit
			}
			return f, nil
		case "ctrl+s":
			return f.submit()
		case "enter":
			if f.Focus == len(f.Inputs)-1 {
				return f.submit()
			}
			fallthrough
		case "tab", "down":
			f.Inputs[f.Focus].Blur()
			f.Focus = (f.Focus + 1) % len(f.Inputs)
			return f, f.Inputs[f.Focus].Focus()
		case "shift+tab", "up":
			f.Inputs[f.Focus].Blur()
			f.Focus = (f.Focus + len(f.Inputs) - 1) % len(f.Inputs)
			return f, f.Inputs[f.Focus].Focus()
		}
	}
	var cmd tea.Cmd
	previous := f.Inputs[f.Focus].Value()
	f.Inputs[f.Focus], cmd = f.Inputs[f.Focus].Update(msg)
	if f.Inputs[f.Focus].Value() != previous && (f.InvalidField < 0 || (f.InvalidField == f.Focus && strings.TrimSpace(f.Inputs[f.Focus].Value()) != "")) {
		f.Error = ""
		f.InvalidField = -1
	}
	return f, cmd
}

func RunForm(ctx context.Context, f Form, in io.Reader, out io.Writer) ([]string, bool, error) {
	f.Standalone = true
	model, err := tea.NewProgram(f, tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return nil, false, err
	}
	done := model.(Form)
	return done.Values(), done.Submitted, nil
}
