package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erikwco/projectory/internal/store"
)

type navigationCatalog struct {
	t             *testing.T
	dir, database string
	paths         map[string]string
}

// newNavigationCatalog registers fictitious projects in an isolated catalog.
func newNavigationCatalog(t *testing.T) navigationCatalog {
	dir := t.TempDir()
	t.Setenv("PROJECTORY_CONFIG_DIR", filepath.Join(dir, "config"))
	t.Setenv("PROJECTORY_DATA_DIR", filepath.Join(dir, "unused-default"))
	c := navigationCatalog{t: t, dir: dir, database: filepath.Join(dir, "catalog.db"), paths: map[string]string{}}
	for _, project := range []struct {
		key, folder string
		args        []string
	}{
		{"shop", "mi tienda", []string{"--name", "Mi Tienda"}},
		{"old", "tienda vieja", []string{"--name", "Tienda Vieja"}},
		{"reserved", "reserved", []string{"--name", "add"}},
		{"dash", "guion", []string{"--name", "-guion"}},
		{"blog", "blog", []string{"--name", "Blog", "--label", "principal"}},
		{"blog-copy", "blog copia", []string{"--project", "Blog", "--label", "copia"}},
	} {
		path := filepath.Join(dir, project.folder)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		c.paths[project.key] = resolved
		if _, err := c.run(append([]string{"add", path, "--no-worktrees"}, project.args...)...); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func (c navigationCatalog) run(args ...string) (string, error) {
	c.t.Helper()
	var out bytes.Buffer
	cmd := New("test")
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--db", c.database}, args...))
	err := cmd.Execute()
	return out.String(), err
}

// navigate runs args with a selection file, as the shell function does, and
// returns the selected path, if any.
func (c navigationCatalog) navigate(args ...string) (string, error) {
	c.t.Helper()
	selection := filepath.Join(c.t.TempDir(), "selection")
	output, err := c.run(append([]string{"--selection-file=" + selection}, args...)...)
	data, readErr := os.ReadFile(selection)
	if os.IsNotExist(readErr) {
		if err == nil {
			err = errors.New("no selection: " + output)
		}
		return "", err
	}
	if err != nil {
		c.t.Fatalf("%v wrote a selection despite failing: %v", args, err)
	}
	fields := strings.Split(string(data), "\x00")
	if len(fields) != 4 || fields[0] == "" || fields[2] != c.database {
		c.t.Fatalf("malformed selection %q", data)
	}
	return fields[1], nil
}

func TestBareAndGoNavigationShareExactRules(t *testing.T) {
	c := newNavigationCatalog(t)
	locations, err := c.run("list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	id := locations[strings.Index(locations, `"id": "`)+7:][:12]
	for _, prefix := range [][]string{nil, {"go"}} {
		for ref, want := range map[string]string{
			"Mi Tienda":       c.paths["shop"],
			"mi tienda":       c.paths["shop"],
			c.paths["old"]:    c.paths["old"],
			"Blog/copia":      c.paths["blog-copy"],
			"tienda vieja":    c.paths["old"],
			"Tienda Vieja/":   "",
			"Mi   Tienda":     "",
			"blog/principal":  c.paths["blog"],
			"Blog/principal ": "",
		} {
			got, err := c.navigate(append(append([]string{}, prefix...), ref)...)
			if want == "" {
				if err == nil {
					t.Errorf("%v %q navigated to %s", prefix, ref, got)
				}
				continue
			}
			if err != nil || got != want {
				t.Errorf("%v %q = %q, %v; want %q", prefix, ref, got, err, want)
			}
		}
		if got, err := c.navigate(append(append([]string{}, prefix...), id)...); err != nil || got == "" {
			t.Errorf("%v by ID: %q %v", prefix, got, err)
		}
	}
}

func TestReservedNamesAndDashesNeedExplicitNavigation(t *testing.T) {
	c := newNavigationCatalog(t)
	// The subcommand wins: "add" without a path is the registration command.
	if _, err := c.navigate("add"); err == nil || !strings.Contains(err.Error(), "indica una ruta") {
		t.Fatalf("reserved word did not run the subcommand: %v", err)
	}
	for _, args := range [][]string{{"go", "add"}, {"--", "add"}} {
		if got, err := c.navigate(args...); err != nil || got != c.paths["reserved"] {
			t.Errorf("%v = %q, %v", args, got, err)
		}
	}
	if _, err := c.navigate("-guion"); err == nil || !strings.Contains(err.Error(), "unknown shorthand flag") {
		t.Fatalf("a dash name must be read as flags without --: %v", err)
	}
	for _, args := range [][]string{{"--", "-guion"}, {"go", "--", "-guion"}} {
		if got, err := c.navigate(args...); err != nil || got != c.paths["dash"] {
			t.Errorf("%v = %q, %v", args, got, err)
		}
	}
}

func TestNonInteractiveNavigationNeverGuesses(t *testing.T) {
	c := newNavigationCatalog(t)
	for _, tc := range []struct {
		args    []string
		message string
	}{
		{[]string{"tien"}, `"tien" no es un nombre exacto; coincide con "Tienda Vieja", "Mi Tienda"`},
		{[]string{"go", "blgo"}, `"blgo" no es un nombre exacto; coincide con "Blog"`},
		{[]string{"Blog"}, `"Blog" tiene varias ubicaciones`},
		{[]string{"go", "blog"}, `"blog" tiene varias ubicaciones`},
		{[]string{"zzzz"}, `ningún proyecto coincide con "zzzz"`},
		{[]string{"lsit"}, "¿querías el comando list?"},
		{[]string{"lsit", "--plain"}, "no se combina con un proyecto; usa pt list --plain; ¿querías el comando list?"},
		{[]string{"Mi", "Tienda"}, `escribe entre comillas los nombres con espacios: pt "Mi Tienda"`},
		{[]string{"go", "Mi", "Tienda"}, "accepts 1 arg"},
	} {
		got, err := c.navigate(tc.args...)
		if err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Errorf("%v: got %q, %v; want error containing %q", tc.args, got, err, tc.message)
		}
	}
	if _, err := c.navigate("zzzz"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("no match must be ErrNotFound: %v", err)
	}
}

func TestListFlagsDoNotCombineWithAProject(t *testing.T) {
	c := newNavigationCatalog(t)
	for _, args := range [][]string{{"Mi Tienda", "--query", "x"}, {"--json", "Mi Tienda"}, {"--plain", "Mi Tienda"}, {"--tag", "web", "Mi Tienda"}, {"--sort", "name", "Mi Tienda"}} {
		if _, err := c.navigate(args...); err == nil || !strings.Contains(err.Error(), "no se combina con un proyecto") {
			t.Errorf("%v: %v", args, err)
		}
	}
	if output, err := c.run("--json", "--query", "tienda"); err != nil || !strings.Contains(output, "Mi Tienda") {
		t.Fatalf("listing flags stopped working without a project: %v %s", err, output)
	}
}

func TestAdministrativeCommandsStayExact(t *testing.T) {
	c := newNavigationCatalog(t)
	for _, args := range [][]string{{"edit", "tien", "--name", "Changed"}, {"remove", "tien", "--yes"}, {"edit", "Blog", "--name", "Changed"}} {
		if _, err := c.run(args...); err == nil {
			t.Errorf("%v acted on an approximate or ambiguous name", args)
		}
	}
	output, err := c.run("list", "--json")
	if err != nil || strings.Contains(output, "Changed") || !strings.Contains(output, "Tienda Vieja") {
		t.Fatalf("catalog changed: %v %s", err, output)
	}
}

func TestCatalogFailureIsNotReportedAsNoMatch(t *testing.T) {
	c := newNavigationCatalog(t)
	c.database = t.TempDir() // A folder cannot be opened as SQLite.
	_, err := c.navigate("Mi Tienda")
	if err == nil || errors.Is(err, store.ErrNotFound) || strings.Contains(err.Error(), "coincide") {
		t.Fatalf("catalog failure looked like a missing project: %v", err)
	}
}
