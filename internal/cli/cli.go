package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/erikwco/projectory/internal/app"
	"github.com/erikwco/projectory/internal/catalog"
	"github.com/erikwco/projectory/internal/config"
	"github.com/erikwco/projectory/internal/shell"
	"github.com/erikwco/projectory/internal/store"
	"github.com/erikwco/projectory/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type settings struct{ Database, SelectionFile string }
type listOptions struct {
	Query, Tag, Sort string
	Plain, JSON      bool
}

// ErrCancelled reports that the person closed a prompt without choosing. The
// command fails quietly so that `pt nombre && comando` does not continue.
var ErrCancelled = errors.New("selección cancelada")

func isTerminal(stream any) bool {
	file, ok := stream.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

// interactive reports whether a prompt can be drawn and answered.
func interactive(cmd *cobra.Command) bool {
	return isTerminal(cmd.InOrStdin()) && isTerminal(cmd.OutOrStdout()) && isTerminal(cmd.ErrOrStderr())
}

func (s *settings) withApp(cmd *cobra.Command, run func(context.Context, *app.App) error) error {
	path := s.Database
	if path == "" {
		var err error
		path, err = store.DefaultPath()
		if err != nil {
			return err
		}
	}
	db, err := store.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	return run(cmd.Context(), &app.App{Store: db})
}

func New(version string) *cobra.Command {
	s := &settings{}
	root := &cobra.Command{Use: "projectory [proyecto]", Short: "Tus proyectos, copias y worktrees en un solo catálogo", Long: "Tus proyectos, copias y worktrees en un solo catálogo.\n\nSin argumentos abre el selector. Con un nombre, ID, ruta o proyecto/etiqueta entra a esa ubicación\ncomo «go». Los subcomandos tienen prioridad: usa «go nombre» o «-- nombre» para un proyecto\nque se llame como un subcomando o empiece por guion.", Version: version, SilenceUsage: true, SilenceErrors: true}
	root.SetIn(os.Stdin)
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().StringVar(&s.Database, "db", "", "ruta del catálogo SQLite (opcional)")
	root.PersistentFlags().StringVar(&s.SelectionFile, "selection-file", "", "protocolo de integración con el shell")
	_ = root.PersistentFlags().MarkHidden("selection-file")
	rootList := &listOptions{}
	listFlags(root, rootList)
	// A validator of its own stops Cobra from rejecting unknown words as
	// commands; subcommands still win because Cobra dispatches them first.
	root.Args = func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			return withSuggestion(cmd, args[0], fmt.Errorf("indica un solo proyecto; escribe entre comillas los nombres con espacios: pt %q", strings.Join(args, " ")))
		}
		return nil
	}
	root.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return s.list(cmd, rootList, false)
		}
		for _, flag := range []string{"query", "tag", "sort", "json", "plain"} {
			if cmd.Flags().Changed(flag) {
				return withSuggestion(cmd, args[0], fmt.Errorf("--%s filtra el listado y no se combina con un proyecto; usa pt list --%s", flag, flag))
			}
		}
		err := s.navigate(cmd, args[0])
		if errors.Is(err, store.ErrNotFound) {
			return withSuggestion(cmd, args[0], err)
		}
		return err
	}
	root.AddCommand(s.add(), s.listCommand(false), s.listCommand(true), s.goCommand(), s.edit(), s.remove(), s.refresh(), s.init(), s.visit(), s.backup(), s.info(), settingsCommand(), updateCommand(version))
	return root
}

func listFlags(cmd *cobra.Command, options *listOptions) {
	cmd.Flags().StringVarP(&options.Query, "query", "q", "", "filtrar nombre, ruta, rama, tecnologías o tags")
	cmd.Flags().StringVar(&options.Tag, "tag", "", "filtrar una categoría exacta")
	cmd.Flags().StringVar(&options.Sort, "sort", "", "orden: added, visited, visits, name; usa el predeterminado de ajustes")
	cmd.Flags().BoolVar(&options.Plain, "plain", false, "imprimir una tabla sin interfaz interactiva")
	cmd.Flags().BoolVar(&options.JSON, "json", false, "imprimir registros como JSON")
}

func settingsCommand() *cobra.Command {
	var searchOnOpen bool
	var defaultSort string
	cmd := &cobra.Command{Use: "settings", Short: "Ajustar las preferencias de navegación", Args: cobra.NoArgs}
	cmd.Flags().BoolVar(&searchOnOpen, "search-on-open", false, "enfocar la búsqueda al abrir el selector o dashboard")
	cmd.Flags().StringVar(&defaultSort, "default-sort", "", "orden predeterminado: added, visited, visits, name")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		path, err := config.DefaultPath()
		if err != nil {
			return err
		}
		values, err := config.Load(path)
		if err != nil {
			return err
		}
		if cmd.Flags().Changed("search-on-open") || cmd.Flags().Changed("default-sort") {
			if cmd.Flags().Changed("search-on-open") {
				values.SearchOnOpen = searchOnOpen
			}
			if cmd.Flags().Changed("default-sort") {
				if !catalog.ValidOrder(defaultSort) {
					return fmt.Errorf("orden desconocido: %s", defaultSort)
				}
				values.DefaultSort = defaultSort
			}
			if err := config.Save(path, values); err != nil {
				return err
			}
		} else if interactive(cmd) {
			return tui.RunSettings(cmd.Context(), tui.NewSettings(path, values), cmd.InOrStdin(), cmd.ErrOrStderr())
		}
		state := "desactivado"
		if values.SearchOnOpen {
			state = "activado"
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Buscar al abrir: %s\nOrden predeterminado: %s\nAjustes: %s\n", state, values.SortOrder(), path)
		return err
	}
	return cmd
}

func (s *settings) selectLocation(cmd *cobra.Command, l *catalog.Location) error {
	if l == nil {
		return nil
	}
	if _, err := catalog.NormalizePath(l.Path, true); err != nil {
		return fmt.Errorf("carpeta no disponible: %w", err)
	}
	if s.SelectionFile != "" {
		database := s.Database
		if database == "" {
			var err error
			database, err = store.DefaultPath()
			if err != nil {
				return err
			}
		}
		database, err := filepath.Abs(database)
		if err != nil {
			return err
		}
		return os.WriteFile(s.SelectionFile, []byte(string(l.ID)+"\x00"+l.Path+"\x00"+database+"\x00"), 0600)
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), l.Path)
	if isTerminal(cmd.OutOrStdout()) {
		fmt.Fprintln(cmd.ErrOrStderr(), "Para cambiar el directorio del shell, activa: eval \"$(projectory init zsh)\" y usa pt.")
	}
	return err
}

func (s *settings) list(cmd *cobra.Command, options *listOptions, dash bool) error {
	if cmd.Flags().Changed("sort") && !catalog.ValidOrder(options.Sort) {
		return fmt.Errorf("orden desconocido: %s", options.Sort)
	}
	if options.JSON && options.Plain {
		return fmt.Errorf("elige --json o --plain")
	}
	path, err := config.DefaultPath()
	if err != nil {
		return err
	}
	values, err := config.Load(path)
	if err != nil {
		return err
	}
	order := values.SortOrder()
	if cmd.Flags().Changed("sort") {
		order = options.Sort
	}
	return s.withApp(cmd, func(ctx context.Context, a *app.App) error {
		if options.Plain || options.JSON || !interactive(cmd) {
			locations, err := a.Store.List(ctx)
			if err != nil {
				return err
			}
			catalog.Sort(locations, order)
			locations = catalog.Filter(locations, options.Query, options.Tag)
			if options.JSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(locations)
			}
			return printLocations(cmd.OutOrStdout(), locations)
		}
		selection, err := tui.RunBrowser(ctx, a, dash, options.Query, options.Tag, order, tui.NewSettings(path, values), cmd.InOrStdin(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		return s.selectLocation(cmd, selection)
	})
}

func printLocations(out io.Writer, locations []catalog.Location) error {
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tPROYECTO\tETIQUETA\tTIPO\tRAMA\tVISITAS\tESTADO\tRUTA"); err != nil {
		return err
	}
	for _, l := range locations {
		state := "disponible"
		if !l.Available {
			state = "no disponible"
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n", l.ID, tui.Clean(l.Project.Name), tui.Clean(l.Label), l.Kind, tui.Clean(l.Branch), l.Visits, state, tui.Clean(l.Path)); err != nil {
			return err
		}
	}
	return w.Flush()
}

func (s *settings) listCommand(dash bool) *cobra.Command {
	name, help := "list", "Buscar una ubicación y entrar a su carpeta"
	if dash {
		name, help = "dash", "Administrar ubicaciones, proyectos y estadísticas"
	}
	options := &listOptions{}
	cmd := &cobra.Command{Use: name, Short: help, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error { return s.list(cmd, options, dash) }}
	listFlags(cmd, options)
	return cmd
}

func (s *settings) add() *cobra.Command {
	var current, noInput, noWorktrees bool
	var input app.AddInput
	var tags string
	cmd := &cobra.Command{Use: "add [ruta]", Short: "Registrar una carpeta y sus worktrees", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if current && len(args) > 0 {
			return fmt.Errorf("usa una ruta o --current")
		}
		if current {
			var err error
			input.Path, err = os.Getwd()
			if err != nil {
				return err
			}
		} else if len(args) > 0 {
			input.Path = args[0]
		}
		interactive := !noInput && input.Name == "" && input.Project == "" && isTerminal(cmd.InOrStdin()) && isTerminal(cmd.ErrOrStderr())
		if interactive {
			fields := []tui.Field{}
			if !current {
				fields = append(fields, tui.Field{Label: "Ruta", Value: input.Path, Required: true})
			}
			suggested := ""
			if input.Path != "" {
				suggested = filepath.Base(input.Path)
			}
			fields = append(fields, tui.Field{Label: "Nombre del proyecto", Value: suggested, Required: true}, tui.Field{Label: "Descripción", Value: input.Description, Hint: "opcional"}, tui.Field{Label: "Tags", Value: tags, Hint: "personal, cliente, web"}, tui.Field{Label: "Etiqueta de esta ubicación", Value: input.Label, Hint: "principal, copia 2025…"}, tui.Field{Label: "Proyecto existente", Hint: "nombre o ID, opcional"})
			values, submitted, err := tui.RunForm(cmd.Context(), tui.NewForm("Registrar proyecto", "Los worktrees vinculados se descubren automáticamente.", fields), cmd.InOrStdin(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if !submitted {
				return nil
			}
			if !current {
				input.Path = values[0]
				values = values[1:]
			}
			input.Name, input.Description, tags, input.Label, input.Project = values[0], values[1], values[2], values[3], values[4]
		}
		if input.Path == "" {
			return fmt.Errorf("indica una ruta o usa --current")
		}
		input.Tags = catalog.Tags(tags)
		input.Discover = !noWorktrees
		return s.withApp(cmd, func(ctx context.Context, a *app.App) error {
			result, err := a.Add(ctx, input)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d ubicaciones agregadas. ID: %s\n", result.Added, result.Primary)
			for _, path := range result.Skipped {
				fmt.Fprintln(cmd.ErrOrStderr(), "Ya pertenece a otro proyecto:", tui.Clean(path))
			}
			return nil
		})
	}}
	cmd.Flags().BoolVar(&current, "current", false, "usar la carpeta actual")
	cmd.Flags().StringVar(&input.Name, "name", "", "nombre del proyecto; omite el formulario")
	cmd.Flags().StringVar(&input.Description, "description", "", "descripción opcional")
	cmd.Flags().StringVar(&input.Project, "project", "", "agrupar bajo un proyecto existente, por nombre o ID")
	cmd.Flags().StringVar(&input.Label, "label", "", "nombre para distinguir esta ubicación")
	cmd.Flags().StringVar(&tags, "tags", "", "categorías separadas por comas")
	cmd.Flags().BoolVar(&noInput, "no-input", false, "usar los valores dados sin formulario")
	cmd.Flags().BoolVar(&noWorktrees, "no-worktrees", false, "registrar solo esta carpeta")
	return cmd
}

// withSuggestion names the subcommand a mistyped word probably meant, so a
// typo such as "lsit" is not silently treated only as a project.
func withSuggestion(cmd *cobra.Command, word string, err error) error {
	var suggestions []string
	for _, sub := range cmd.Commands() {
		if !sub.IsAvailableCommand() {
			continue
		}
		for _, name := range append([]string{sub.Name()}, sub.Aliases...) {
			if catalog.Resembles(word, name) {
				suggestions = append(suggestions, sub.Name())
				break
			}
		}
	}
	if len(suggestions) > 0 {
		return fmt.Errorf("%w; ¿querías el comando %s?", err, strings.Join(suggestions, ", "))
	}
	return err
}

// navigate enters an exactly named location directly. Approximate names and
// names with several locations are confirmed in a prompt, never chosen alone.
func (s *settings) navigate(cmd *cobra.Command, ref string) error {
	return s.withApp(cmd, func(ctx context.Context, a *app.App) error {
		locations, err := a.Store.List(ctx)
		if err != nil {
			return err
		}
		found := catalog.Navigate(locations, ref)
		switch {
		case len(found.Locations) == 0:
			return fmt.Errorf("%w: ningún proyecto coincide con %q", store.ErrNotFound, ref)
		case found.Exact && len(found.Locations) == 1:
			return s.selectLocation(cmd, &found.Locations[0])
		case !interactive(cmd) && found.Exact:
			return fmt.Errorf("%q tiene varias ubicaciones; usa su ID, ruta o proyecto/etiqueta (pt list --plain los muestra)", ref)
		case !interactive(cmd):
			return fmt.Errorf("%q no es un nombre exacto; coincide con %s. Usa el nombre exacto, el ID o la ruta", ref, projectNames(found.Locations))
		}
		choice, err := tui.RunMatches(ctx, tui.NewMatches(ref, found), cmd.InOrStdin(), cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		if choice == nil {
			return ErrCancelled
		}
		return s.selectLocation(cmd, choice)
	})
}

func projectNames(locations []catalog.Location) string {
	const shown = 5
	var names []string
	seen := map[catalog.ProjectID]bool{}
	for _, l := range locations {
		if !seen[l.Project.ID] {
			seen[l.Project.ID] = true
			names = append(names, strconv.Quote(tui.Clean(l.Project.Name)))
		}
	}
	if len(names) > shown {
		return strings.Join(names[:shown], ", ") + fmt.Sprintf(" y %d más", len(names)-shown)
	}
	return strings.Join(names, ", ")
}

func (s *settings) goCommand() *cobra.Command {
	return &cobra.Command{Use: "go <nombre|id|ruta|proyecto/etiqueta>", Short: "Entrar a una ubicación por nombre, ID o ruta", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return s.navigate(cmd, args[0])
	}}
}

func (s *settings) edit() *cobra.Command {
	var name, description, label, tags, path string
	cmd := &cobra.Command{Use: "edit <id|ruta|proyecto/etiqueta>", Short: "Editar metadatos o corregir una ruta", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return s.withApp(cmd, func(ctx context.Context, a *app.App) error {
			l, err := a.Store.Resolve(ctx, args[0])
			if err != nil {
				return err
			}
			newPath := l.Path
			changed := false
			for _, flag := range []string{"name", "description", "label", "tags", "path"} {
				changed = changed || cmd.Flags().Changed(flag)
			}
			if !changed {
				if !isTerminal(cmd.InOrStdin()) || !isTerminal(cmd.ErrOrStderr()) {
					return fmt.Errorf("indica --name, --description, --label, --tags o --path")
				}
				form := tui.NewForm("Editar ubicación", "Nombre, descripción y tags pertenecen al proyecto compartido.", []tui.Field{{Label: "Nombre", Value: l.Project.Name, Required: true}, {Label: "Descripción", Value: l.Project.Description}, {Label: "Etiqueta", Value: l.Label}, {Label: "Tags", Value: strings.Join(l.Project.Tags, ", ")}, {Label: "Ruta", Value: l.Path, Required: true}})
				values, submitted, err := tui.RunForm(ctx, form, cmd.InOrStdin(), cmd.ErrOrStderr())
				if err != nil {
					return err
				}
				if !submitted {
					return nil
				}
				l.Project.Name, l.Project.Description, l.Label, newPath = values[0], values[1], values[2], values[4]
				l.Project.Tags = catalog.Tags(values[3])
			} else {
				if cmd.Flags().Changed("name") {
					l.Project.Name = name
				}
				if cmd.Flags().Changed("description") {
					l.Project.Description = description
				}
				if cmd.Flags().Changed("label") {
					l.Label = label
				}
				if cmd.Flags().Changed("tags") {
					l.Project.Tags = catalog.Tags(tags)
				}
				if cmd.Flags().Changed("path") {
					newPath = path
				}
			}
			if err = a.Edit(ctx, l, newPath); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Ubicación actualizada.")
			return nil
		})
	}}
	cmd.Flags().StringVar(&name, "name", "", "nombre del proyecto compartido")
	cmd.Flags().StringVar(&description, "description", "", "descripción del proyecto")
	cmd.Flags().StringVar(&label, "label", "", "etiqueta de la ubicación")
	cmd.Flags().StringVar(&tags, "tags", "", "tags del proyecto, separados por comas")
	cmd.Flags().StringVar(&path, "path", "", "nueva ruta de esta ubicación; conserva sus visitas")
	return cmd
}

func (s *settings) remove() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{Use: "remove <id|ruta|proyecto/etiqueta>", Aliases: []string{"rm"}, Short: "Quitar una ubicación del catálogo conservando los archivos", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return s.withApp(cmd, func(ctx context.Context, a *app.App) error {
			l, err := a.Store.Resolve(ctx, args[0])
			if err != nil {
				return err
			}
			if !yes {
				if !isTerminal(cmd.InOrStdin()) || !isTerminal(cmd.ErrOrStderr()) {
					return fmt.Errorf("usa --yes para quitar del catálogo; los archivos se conservan")
				}
				form := tui.NewForm("Quitar del catálogo", tui.Clean(l.Path)+" · los archivos se conservarán", []tui.Field{{Label: "Escribe quitar para confirmar", Required: true}})
				values, submitted, err := tui.RunForm(ctx, form, cmd.InOrStdin(), cmd.ErrOrStderr())
				if err != nil {
					return err
				}
				if !submitted || values[0] != "quitar" {
					return nil
				}
			}
			if err = a.Store.Remove(ctx, l.ID); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Ubicación quitada del catálogo. Archivos conservados.")
			return nil
		})
	}}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirmar sin formulario")
	return cmd
}

func (s *settings) refresh() *cobra.Command {
	return &cobra.Command{Use: "refresh", Short: "Actualizar tecnologías, ramas y worktrees de repositorios conocidos", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return s.withApp(cmd, func(ctx context.Context, a *app.App) error {
			r, err := a.Refresh(ctx)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d revisadas · %d nuevas · %d no disponibles\n", r.Checked, r.Added, r.Missing)
			for _, warning := range r.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), tui.Clean(warning))
			}
			return nil
		})
	}}
}

func (s *settings) init() *cobra.Command {
	var command string
	cmd := &cobra.Command{Use: "init <zsh|bash>", Short: "Generar la función del shell: eval \"$(projectory init zsh)\"", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		binary, err := os.Executable()
		if err != nil {
			return err
		}
		code, err := shell.Init(args[0], command, shell.ExecutablePath(os.Args[0], binary))
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), code)
		return err
	}}
	cmd.Flags().StringVar(&command, "cmd", "pt", "nombre de la función del shell")
	return cmd
}

func (s *settings) visit() *cobra.Command {
	var path string
	cmd := &cobra.Command{Use: "_visit <id>", Hidden: true, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		resolved, err := catalog.NormalizePath(path, true)
		if err != nil {
			return err
		}
		return s.withApp(cmd, func(ctx context.Context, a *app.App) error {
			return a.Store.Visit(ctx, catalog.LocationID(args[0]), resolved)
		})
	}}
	cmd.Flags().StringVar(&path, "path", "", "directorio confirmado por el shell")
	_ = cmd.MarkFlagRequired("path")
	return cmd
}

func (s *settings) backup() *cobra.Command {
	return &cobra.Command{Use: "backup <archivo.db>", Short: "Crear una copia consistente del catálogo SQLite", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return s.withApp(cmd, func(ctx context.Context, a *app.App) error {
			path, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			if err = a.Store.Backup(ctx, path); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		})
	}}
}

func (s *settings) info() *cobra.Command {
	return &cobra.Command{Use: "info", Short: "Mostrar la ruta del catálogo y estadísticas", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return s.withApp(cmd, func(ctx context.Context, a *app.App) error {
			locations, err := a.Store.List(ctx)
			if err != nil {
				return err
			}
			projects := map[catalog.ProjectID]bool{}
			var visits int64
			missing := 0
			for _, l := range locations {
				projects[l.Project.ID] = true
				visits += l.Visits
				if !l.Available {
					missing++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Catálogo: %s\nProyectos: %d\nUbicaciones: %d\nVisitas: %d\nNo disponibles: %d\n", a.Store.Path, len(projects), len(locations), visits, missing)
			return nil
		})
	}}
}
