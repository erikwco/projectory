package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erikwco/projectory/internal/catalog"
	"github.com/erikwco/projectory/internal/config"
)

func TestSettingsCommandPersistsWithoutOpeningCatalog(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROJECTORY_CONFIG_DIR", filepath.Join(dir, "config"))
	database := filepath.Join(dir, "unused.db")
	for _, enabled := range []bool{true, false} {
		flag, expected := "--search-on-open=true", "activado"
		if !enabled {
			flag, expected = "--search-on-open=false", "desactivado"
		}
		var out bytes.Buffer
		cmd := New("test")
		cmd.SetIn(strings.NewReader(""))
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"settings", flag, "--db", database})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Buscar al abrir: "+expected) {
			t.Fatalf("wrong settings output: %s", out.String())
		}
		values, err := config.Load(filepath.Join(dir, "config", "settings.json"))
		if err != nil || values.SearchOnOpen != enabled {
			t.Fatalf("setting was not saved: %+v %v", values, err)
		}
	}
	if _, err := os.Stat(database); !os.IsNotExist(err) {
		t.Fatal("settings unexpectedly opened a project catalog")
	}
}

func TestSavedOrderAppliesToAllListsAndExplicitFlagWins(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROJECTORY_CONFIG_DIR", filepath.Join(dir, "config"))
	database := filepath.Join(dir, "catalog.db")
	run := func(args ...string) (string, error) {
		t.Helper()
		var out bytes.Buffer
		cmd := New("test")
		cmd.SetIn(strings.NewReader(""))
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append(args, "--db", database))
		err := cmd.Execute()
		return out.String(), err
	}
	for _, name := range []string{"Alpha", "Zeta"} {
		path := filepath.Join(dir, name)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if _, err := run("add", path, "--name", name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := run("settings", "--default-sort=name", "--search-on-open=true"); err != nil {
		t.Fatal(err)
	}
	for _, prefix := range [][]string{nil, {"list"}, {"dash"}} {
		for _, override := range []bool{false, true} {
			args := append(append([]string{}, prefix...), "--json")
			expected := "Alpha"
			if override {
				args = append(args, "--sort", "added")
				expected = "Zeta"
			}
			output, err := run(args...)
			if err != nil {
				t.Fatal(err)
			}
			var locations []catalog.Location
			if err := json.Unmarshal([]byte(output), &locations); err != nil {
				t.Fatal(err)
			}
			if len(locations) != 2 || locations[0].Project.Name != expected {
				t.Fatalf("unexpected order for %v: %s", args, output)
			}
		}
	}
	if _, err := run("settings", "--search-on-open=false"); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"settings", "--default-sort=invalid", "--search-on-open=true"}, {"list", "--sort=invalid"}, {"list", "--sort="}} {
		if _, err := run(args...); err == nil {
			t.Fatalf("invalid order accepted: %v", args)
		}
	}
	values, err := config.Load(filepath.Join(dir, "config", "settings.json"))
	if err != nil || values.SortOrder() != "name" || values.SearchOnOpen {
		t.Fatalf("explicit flag or failed save changed the preference: %+v %v", values, err)
	}
}
