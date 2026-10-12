package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erikwco/projectory/internal/update"
	"github.com/erikwco/projectory/internal/update/updatetest"
	"github.com/spf13/cobra"
)

const latestURL = updatetest.API + "/releases/latest"

type updateRun struct {
	gh     *updatetest.GitHub
	path   string
	output string
	err    error
}

// runUpdate installs a fake executable reporting version at path (relative to
// a temp dir) and runs the update command against the fake GitHub.
func runUpdate(t *testing.T, gh *updatetest.GitHub, version, path string, tty bool, stdin string, args ...string) updateRun {
	t.Helper()
	path = filepath.Join(t.TempDir(), path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(updatetest.Binary(version)), 0o755); err != nil {
		t.Fatal(err)
	}
	u := &update.Updater{Version: version, Executable: path, GOOS: "darwin", GOARCH: "arm64", Transport: gh.Transport()}
	var out bytes.Buffer
	cmd := newUpdateCommand(u, func(*cobra.Command) bool { return tty })
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return updateRun{gh, path, out.String(), err}
}

func (r updateRun) installed(t *testing.T, version string) {
	t.Helper()
	data, err := os.ReadFile(r.path)
	if err != nil || string(data) != updatetest.Binary(version) {
		t.Fatalf("executable is %q (%v), want version %s\n%s", data, err, version, r.output)
	}
}

func publishedGitHub(t *testing.T, version string) *updatetest.GitHub {
	gh := updatetest.New(t)
	gh.JSON(latestURL, gh.Release(version, "darwin", "arm64"))
	return gh
}

func TestUpdateCheckReportsWithoutDownloading(t *testing.T) {
	r := runUpdate(t, publishedGitHub(t, "1.2.0"), "1.0.0", "bin/projectory", false, "", "--check")
	if r.err != nil || !strings.Contains(r.output, "Versión actual: 1.0.0") || !strings.Contains(r.output, "Última versión: 1.2.0") ||
		!strings.Contains(r.output, "binario independiente") || !strings.Contains(r.output, "disponible: projectory update") {
		t.Fatalf("unexpected check: %v\n%s", r.err, r.output)
	}
	if r.gh.Downloaded() {
		t.Fatal("--check downloaded assets")
	}
	r.installed(t, "1.0.0")
}

func TestUpdateWithYesReplacesExecutable(t *testing.T) {
	r := runUpdate(t, publishedGitHub(t, "1.2.0"), "1.0.0", "bin/projectory", false, "", "--yes")
	if r.err != nil || !strings.Contains(r.output, "projectory 1.2.0 instalado") {
		t.Fatalf("update failed: %v\n%s", r.err, r.output)
	}
	r.installed(t, "1.2.0")
}

func TestUpdateConfirmation(t *testing.T) {
	r := runUpdate(t, publishedGitHub(t, "1.2.0"), "1.0.0", "bin/projectory", false, "s\n")
	if r.err == nil || !strings.Contains(r.err.Error(), "--yes") || r.gh.Downloaded() {
		t.Fatalf("non-interactive update without --yes: %v\n%s", r.err, r.output)
	}
	r.installed(t, "1.0.0")

	r = runUpdate(t, publishedGitHub(t, "1.2.0"), "1.0.0", "bin/projectory", true, "n\n")
	if r.err != nil || !strings.Contains(r.output, "cancelada") || r.gh.Downloaded() {
		t.Fatalf("declined update: %v\n%s", r.err, r.output)
	}
	r.installed(t, "1.0.0")

	r = runUpdate(t, publishedGitHub(t, "1.2.0"), "1.0.0", "bin/projectory", true, "sí\n")
	if r.err != nil || !strings.Contains(r.output, "[s/N]") {
		t.Fatalf("accepted update: %v\n%s", r.err, r.output)
	}
	r.installed(t, "1.2.0")
}

func TestUpdateHomebrewDelegatesToBrew(t *testing.T) {
	brewPath := "Cellar/projectory/1.0.0/bin/projectory"
	r := runUpdate(t, publishedGitHub(t, "1.2.0"), "1.0.0", brewPath, true, "s\n", "--yes")
	if r.err == nil || !strings.Contains(r.err.Error(), update.BrewUpgrade) || len(r.gh.Requests()) != 0 {
		t.Fatalf("brew update: %v requests=%v", r.err, r.gh.Requests())
	}
	r.installed(t, "1.0.0")

	r = runUpdate(t, publishedGitHub(t, "1.2.0"), "1.0.0", brewPath, false, "", "--check")
	if r.err != nil || !strings.Contains(r.output, "Instalación: Homebrew") || !strings.Contains(r.output, update.BrewUpgrade) || r.gh.Downloaded() {
		t.Fatalf("brew check: %v\n%s", r.err, r.output)
	}
}

func TestUpdateDevelopmentBuildIsNeverReplaced(t *testing.T) {
	r := runUpdate(t, publishedGitHub(t, "1.2.0"), "1.0.0-dev", "bin/projectory", true, "s\n", "--yes")
	if r.err == nil || !strings.Contains(r.err.Error(), "compilación de desarrollo") || len(r.gh.Requests()) != 0 {
		t.Fatalf("dev update: %v requests=%v", r.err, r.gh.Requests())
	}
	r = runUpdate(t, publishedGitHub(t, "1.2.0"), "1.0.0-dev", "bin/projectory", false, "", "--check")
	if r.err != nil || !strings.Contains(r.output, "no se comparan ni se reemplazan") {
		t.Fatalf("dev check: %v\n%s", r.err, r.output)
	}
	r.installed(t, "1.0.0-dev")
}

func TestUpdateNeverDowngrades(t *testing.T) {
	for _, current := range []string{"1.2.0", "1.3.0-rc.1"} {
		r := runUpdate(t, publishedGitHub(t, "1.2.0"), current, "bin/projectory", false, "", "--yes")
		if r.err != nil || r.gh.Downloaded() || !(strings.Contains(r.output, "última versión") || strings.Contains(r.output, "más reciente")) {
			t.Fatalf("%s: %v\n%s", current, r.err, r.output)
		}
		r.installed(t, current)
	}
}

func TestUpdatePrereleaseChannel(t *testing.T) {
	gh := updatetest.New(t)
	gh.Handle(latestURL, updatetest.Response{Status: 404})
	r := runUpdate(t, gh, "1.0.0-rc.1", "bin/projectory", false, "", "--check")
	if r.err == nil || !strings.Contains(r.err.Error(), "--prerelease") {
		t.Fatalf("no stable release: %v", r.err)
	}

	gh = updatetest.New(t)
	gh.JSON(updatetest.API+"/releases?per_page=100", []any{gh.Release("1.0.0-rc.1", "darwin", "arm64"), gh.Release("1.0.0-rc.2", "darwin", "arm64")})
	r = runUpdate(t, gh, "1.0.0-rc.1", "bin/projectory", false, "", "--prerelease", "--yes")
	if r.err != nil {
		t.Fatalf("prerelease update: %v\n%s", r.err, r.output)
	}
	r.installed(t, "1.0.0-rc.2")

	// A release candidate upgrades to the matching stable release on the default channel.
	r = runUpdate(t, publishedGitHub(t, "1.0.0"), "1.0.0-rc.2", "bin/projectory", false, "", "--yes")
	if r.err != nil {
		t.Fatalf("rc to stable: %v\n%s", r.err, r.output)
	}
	r.installed(t, "1.0.0")
}

func TestUpdateCancellationKeepsExecutable(t *testing.T) {
	gh := publishedGitHub(t, "1.2.0")
	path := filepath.Join(t.TempDir(), "projectory")
	if err := os.WriteFile(path, []byte(updatetest.Binary("1.0.0")), 0o755); err != nil {
		t.Fatal(err)
	}
	u := &update.Updater{Version: "1.0.0", Executable: path, GOOS: "darwin", GOARCH: "arm64", Transport: gh.Transport()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := newUpdateCommand(u, func(*cobra.Command) bool { return false })
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"--yes"})
	if err := cmd.ExecuteContext(ctx); err == nil {
		t.Fatal("cancelled update succeeded")
	}
	if data, _ := os.ReadFile(path); string(data) != updatetest.Binary("1.0.0") {
		t.Fatal("executable changed")
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Fatalf("temporary files left: %v", entries)
	}
}

func TestUpdateIsRegisteredAndNeverOpensCatalog(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROJECTORY_CONFIG_DIR", filepath.Join(dir, "config"))
	database := filepath.Join(dir, "catalog.db")
	cmd := New("1.0.0-dev")
	var out bytes.Buffer
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--yes", "--db", database})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "compilación de desarrollo") {
		t.Fatalf("update on development build: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("update touched catalog or settings: %v", entries)
	}
}
