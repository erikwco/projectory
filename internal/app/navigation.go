package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/erikwco/projectory/internal/catalog"
	"github.com/erikwco/projectory/internal/store"
)

// MissingLocation fixes the cleanup target; Remove must still recheck absence.
type MissingLocation struct {
	location catalog.Location
	store    *store.Store
	cause    error
}

func (m *MissingLocation) Error() string {
	return fmt.Sprintf("la carpeta ya no existe: %s", m.location.Path)
}

func (m *MissingLocation) Unwrap() error              { return m.cause }
func (m *MissingLocation) Location() catalog.Location { return m.location }

func (a *App) CheckNavigation(l catalog.Location) error {
	if _, err := catalog.NormalizePath(l.Path, true); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &MissingLocation{location: l, store: a.Store, cause: err}
		}
		return fmt.Errorf("carpeta no disponible: %w", err)
	}
	return nil
}

func (m *MissingLocation) Remove(ctx context.Context) error {
	if m == nil || m.store == nil || m.location.ID == "" {
		return errors.New("falta la ubicación que se confirmó")
	}
	locations, err := m.store.List(ctx)
	if err != nil {
		return err
	}
	for _, l := range locations {
		if l.ID != m.location.ID {
			continue
		}
		if l.Path != m.location.Path {
			return errors.New("la ubicación cambió; no se quitó del catálogo")
		}
		if _, err := catalog.NormalizePath(l.Path, true); err == nil {
			return errors.New("la carpeta volvió a estar disponible; no se quitó. Vuelve a abrirla")
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("carpeta no disponible: %w", err)
		}
		return m.store.Remove(ctx, l.ID)
	}
	return fmt.Errorf("%w: %q", store.ErrNotFound, m.location.ID)
}
