// Package config stores user preferences independently of project catalogs.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/erikwco/projectory/internal/catalog"
)

type Settings struct {
	SearchOnOpen bool   `json:"search_on_open"`
	DefaultSort  string `json:"default_sort,omitempty"`
}

// SortOrder preserves registration order for configurations predating this setting.
func (s Settings) SortOrder() string {
	if s.DefaultSort == "" {
		return "added"
	}
	return s.DefaultSort
}

func DefaultPath() (string, error) {
	if dir := os.Getenv("PROJECTORY_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "settings.json"), nil
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "projectory", "settings.json"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "projectory", "settings.json"), nil
}

func Load(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("leer ajustes %s: %w", path, err)
	}
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, fmt.Errorf("ajustes inválidos en %s: %w", path, err)
	}
	if !catalog.ValidOrder(settings.SortOrder()) {
		return Settings{}, fmt.Errorf("ajustes inválidos en %s: orden desconocido: %s", path, settings.DefaultSort)
	}
	return settings, nil
}

// Save replaces the complete file so readers never see a partial JSON document.
func Save(path string, settings Settings) error {
	if !catalog.ValidOrder(settings.SortOrder()) {
		return fmt.Errorf("orden desconocido: %s", settings.DefaultSort)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("crear carpeta de ajustes: %w", err)
	}
	f, err := os.CreateTemp(dir, ".settings-*.json")
	if err != nil {
		return fmt.Errorf("guardar ajustes: %w", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("guardar ajustes: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("guardar ajustes: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("guardar ajustes: %w", err)
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("guardar ajustes: %w", err)
	}
	return nil
}
