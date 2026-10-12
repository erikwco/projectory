package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/erikwco/projectory/internal/update"
	"github.com/spf13/cobra"
)

func updateCommand(version string) *cobra.Command {
	return newUpdateCommand(update.New(version), func(cmd *cobra.Command) bool {
		return isTerminal(cmd.InOrStdin()) && isTerminal(cmd.ErrOrStderr())
	})
}

func newUpdateCommand(u *update.Updater, interactive func(*cobra.Command) bool) *cobra.Command {
	var check, yes, prerelease bool
	cmd := &cobra.Command{Use: "update", Short: "Buscar e instalar la última versión publicada de projectory", Args: cobra.NoArgs}
	cmd.Flags().BoolVar(&check, "check", false, "solo consultar la última versión, sin descargar nada")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "actualizar sin pedir confirmación")
	cmd.Flags().BoolVar(&prerelease, "prerelease", false, "incluir versiones candidatas (-rc)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		inst, err := u.Installation()
		if err != nil {
			return err
		}
		// Installations Projectory does not own are rejected before any network access.
		if !check {
			switch inst.Method {
			case update.Homebrew:
				return fmt.Errorf("Homebrew gestiona esta instalación (%s); actualiza con: %s", inst.Path, update.BrewUpgrade)
			case update.Development:
				return fmt.Errorf("compilación de desarrollo (%s): solo se reemplazan binarios publicados; instala una versión oficial o vuelve a compilar desde el código fuente", u.Version)
			}
		}
		latest, err := u.Latest(cmd.Context(), prerelease)
		if errors.Is(err, update.ErrNoRelease) && !prerelease {
			return errors.New("no hay versiones estables publicadas; usa --prerelease para incluir versiones candidatas")
		}
		if err != nil {
			return err
		}
		location := inst.Path
		if inst.Link != "" {
			location += ", mediante el enlace " + inst.Link
		}
		fmt.Fprintf(out, "Versión actual: %s\nÚltima versión: %s\nInstalación: %s (%s)\n", u.Version, latest.Version, inst.Method, location)
		if inst.Method == update.Development {
			_, err = fmt.Fprintln(out, "Las compilaciones de desarrollo no se comparan ni se reemplazan automáticamente.")
			return err
		}
		switch c := latest.Version.Compare(inst.Current); {
		case c == 0:
			_, err = fmt.Fprintln(out, "Ya tienes la última versión.")
			return err
		case c < 0:
			_, err = fmt.Fprintln(out, "Tu versión es más reciente que la publicada; no se instalan versiones anteriores.")
			return err
		}
		if check {
			if inst.Method == update.Homebrew && latest.Version.Prerelease() {
				_, err = fmt.Fprintln(out, "Hay una versión candidata disponible. La fórmula de Homebrew solo distribuye versiones estables.")
				return err
			}
			next := "projectory update"
			if inst.Method == update.Homebrew {
				next = update.BrewUpgrade
			} else if prerelease {
				next += " --prerelease"
			}
			_, err = fmt.Fprintf(out, "Hay una actualización disponible: %s\n", next)
			return err
		}
		if !yes {
			if !interactive(cmd) {
				return errors.New("se necesita confirmación: vuelve a ejecutar con --yes o desde una terminal interactiva")
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "¿Reemplazar %s con la versión %s? [s/N]: ", inst.Path, latest.Version)
			answer, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			switch strings.ToLower(strings.TrimSpace(answer)) {
			case "s", "si", "sí", "y", "yes":
			default:
				_, err = fmt.Fprintln(out, "Actualización cancelada.")
				return err
			}
		}
		// Interrupting from here on cancels the download and removes temporary files.
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		defer stop()
		fmt.Fprintf(out, "Descargando y verificando projectory %s…\n", latest.Version)
		if err := u.Install(ctx, inst, latest); err != nil {
			return err
		}
		fmt.Fprintf(out, "projectory %s instalado en %s\n", latest.Version, inst.Path)
		if inst.Link != "" {
			fmt.Fprintf(out, "El enlace %s se conserva y apunta a la nueva versión.\n", inst.Link)
		}
		return nil
	}
	return cmd
}
