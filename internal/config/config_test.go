package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erikwco/projectory/internal/catalog"
)

func TestSettingsPersistAcrossLoadsWithoutCreatingFilesForDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences", "settings.json")
	values, err := Load(path)
	if err != nil || values.SearchOnOpen {
		t.Fatalf("missing settings should retain manual search: %+v %v", values, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatal("reading defaults created a preferences directory")
	}
	for _, enabled := range []bool{true, false} {
		if err := Save(path, Settings{SearchOnOpen: enabled}); err != nil {
			t.Fatal(err)
		}
		saved, err := Load(path)
		if err != nil || saved.SearchOnOpen != enabled {
			t.Fatalf("setting did not survive a fresh load: %+v %v", saved, err)
		}
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 || files[0].Name() != "settings.json" {
		t.Fatalf("temporary settings files were left behind: %v %v", files, err)
	}
}

func TestDefaultSortSupportsExistingSettingsAndRejectsInvalidOrders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	for _, data := range []string{`{}`, `{"search_on_open":true}`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		settings, err := Load(path)
		if err != nil || settings.SortOrder() != "added" {
			t.Fatalf("old settings changed the default order: %+v %v", settings, err)
		}
	}
	for _, order := range catalog.OrderKeys() {
		if err := Save(path, Settings{SearchOnOpen: true, DefaultSort: order}); err != nil {
			t.Fatal(err)
		}
		settings, err := Load(path)
		if err != nil || settings.SortOrder() != order || !settings.SearchOnOpen {
			t.Fatalf("sort preference lost another setting: %+v %v", settings, err)
		}
	}
	before, _ := os.ReadFile(path)
	if err := Save(path, Settings{DefaultSort: "invalid"}); err == nil {
		t.Fatal("invalid order was saved")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid order replaced the saved configuration")
	}
	if err := os.WriteFile(path, []byte(`{"default_sort":"invalid"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("invalid order in configuration was silently accepted")
	}
}

func TestSettingsReportInvalidInputAndWriteFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	for _, data := range []string{`{`, `{"search_on_open":"yes"}`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("invalid configuration was silently accepted")
		}
	}
	if err := Save(filepath.Join(path, "settings.json"), Settings{SearchOnOpen: true}); err == nil {
		t.Fatal("saving beneath a file should fail")
	}
}

func TestSettingsPathIsIndependentOfCatalogAndHonorsConfigOverrides(t *testing.T) {
	configDir, xdgDir := t.TempDir(), t.TempDir()
	t.Setenv("PROJECTORY_DATA_DIR", t.TempDir())
	t.Setenv("PROJECTORY_CONFIG_DIR", configDir)
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	path, err := DefaultPath()
	if err != nil || path != filepath.Join(configDir, "settings.json") {
		t.Fatalf("app override was not used: %s %v", path, err)
	}
	t.Setenv("PROJECTORY_CONFIG_DIR", "")
	path, err = DefaultPath()
	if err != nil || path != filepath.Join(xdgDir, "projectory", "settings.json") {
		t.Fatalf("XDG config directory was not used: %s %v", path, err)
	}
}
