package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erikwco/projectory/internal/update/updatetest"
)

const (
	latestURL   = updatetest.API + "/releases/latest"
	releasesURL = updatetest.API + "/releases?per_page=100"
)

func newTestUpdater(t *testing.T, gh *updatetest.GitHub, version string) (*Updater, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "projectory")
	if err := os.WriteFile(path, []byte(updatetest.Binary(version)), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Updater{Version: version, Executable: path, GOOS: "linux", GOARCH: "arm64", Transport: gh.Transport()}, path
}

func TestLatestStableUsesOfficialEndpointAndHeaders(t *testing.T) {
	gh := updatetest.New(t)
	gh.JSON(latestURL, gh.Release("1.2.0", "linux", "arm64"))
	u, _ := newTestUpdater(t, gh, "1.0.0")
	r, err := u.Latest(context.Background(), false)
	if err != nil || r.Version.String() != "1.2.0" {
		t.Fatalf("Latest = %+v, %v", r, err)
	}
	h := gh.Header(latestURL)
	if h.Get("Accept") != "application/vnd.github+json" || h.Get("X-GitHub-Api-Version") != "2026-03-10" || h.Get("User-Agent") != "projectory/1.0.0" {
		t.Fatalf("wrong API headers: %v", h)
	}
	if gh.Downloaded() {
		t.Fatal("checking downloaded release assets")
	}
}

func TestLatestPrereleasePicksNewestSupportedNonDraft(t *testing.T) {
	gh := updatetest.New(t)
	draft := map[string]any{"tag_name": "v9.0.0", "draft": true, "prerelease": false, "assets": []any{}}
	odd := map[string]any{"tag_name": "nightly", "draft": false, "prerelease": true, "assets": []any{}}
	gh.JSON(releasesURL, []any{draft, odd, gh.Release("1.0.0", "linux", "arm64"), gh.Release("1.1.0-rc.2", "linux", "arm64"), gh.Release("1.1.0-rc.10", "linux", "arm64")})
	u, _ := newTestUpdater(t, gh, "1.0.0")
	r, err := u.Latest(context.Background(), true)
	if err != nil || r.Version.String() != "1.1.0-rc.10" {
		t.Fatalf("Latest = %+v, %v", r, err)
	}
}

func TestLatestHTTPFailures(t *testing.T) {
	cases := map[string]struct {
		resp updatetest.Response
		want string
	}{
		"no stable release": {updatetest.Response{Status: http.StatusNotFound, Body: []byte(`{"message":"Not Found"}`)}, ErrNoRelease.Error()},
		"rate limited":      {updatetest.Response{Status: http.StatusForbidden, Header: http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"1900000000"}}}, "limitó las solicitudes"},
		"too many requests": {updatetest.Response{Status: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"60"}}}, "reintenta en 60 s"},
		"forbidden":         {updatetest.Response{Status: http.StatusForbidden}, "403 Forbidden"},
		"server error":      {updatetest.Response{Status: http.StatusBadGateway}, "502"},
		"malformed JSON":    {updatetest.Response{Status: http.StatusOK, Body: []byte(`{"tag_name":`)}, "respuesta de GitHub no válida"},
		"oversized":         {updatetest.Response{Status: http.StatusOK, Body: []byte(strings.Repeat(" ", apiLimit+1))}, "demasiado grande"},
		"bad tag":           {updatetest.Response{Status: http.StatusOK, Body: []byte(`{"tag_name":"latest"}`)}, "etiqueta no soportada"},
		"prerelease tag":    {updatetest.Response{Status: http.StatusOK, Body: []byte(`{"tag_name":"v2.0.0-rc.1"}`)}, "como versión estable"},
		"prerelease flag":   {updatetest.Response{Status: http.StatusOK, Body: []byte(`{"tag_name":"v2.0.0","prerelease":true}`)}, "como versión estable"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			gh := updatetest.New(t)
			gh.Handle(latestURL, c.resp)
			u, _ := newTestUpdater(t, gh, "1.0.0")
			_, err := u.Latest(context.Background(), false)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
		})
	}
}

func TestInstallationDetection(t *testing.T) {
	dir := t.TempDir()
	cellar := filepath.Join(dir, "Cellar", "projectory", "1.0.0", "bin")
	if err := os.MkdirAll(cellar, 0o755); err != nil {
		t.Fatal(err)
	}
	brewBinary := filepath.Join(cellar, "projectory")
	manual := filepath.Join(dir, "manual")
	link := filepath.Join(dir, "link")
	for _, p := range []string{brewBinary, manual} {
		if err := os.WriteFile(p, nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(manual, link); err != nil {
		t.Fatal(err)
	}
	brewLink := filepath.Join(dir, "brewlink")
	if err := os.Symlink(brewBinary, brewLink); err != nil {
		t.Fatal(err)
	}
	keg := filepath.Join(dir, "linuxbrew", "projectory", "1.0.0")
	kegBinary := filepath.Join(keg, "bin", "projectory")
	if err := os.MkdirAll(filepath.Dir(kegBinary), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{kegBinary, filepath.Join(keg, "INSTALL_RECEIPT.json")} {
		if err := os.WriteFile(p, nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	resolve := func(p string) string { r, _ := filepath.EvalSymlinks(p); return r }
	cases := []struct {
		executable, version string
		method              Method
		path, link          string
	}{
		{brewLink, "1.0.0", Homebrew, resolve(brewBinary), brewLink},
		{manual, "0.1.0-dev", Development, resolve(manual), ""},
		{link, "1.0.0", Standalone, resolve(manual), link},
		{kegBinary, "1.0.0", Homebrew, resolve(kegBinary), ""},
	}
	for _, c := range cases {
		inst, err := (&Updater{Version: c.version, Executable: c.executable}).Installation()
		if err != nil || inst.Method != c.method || inst.Path != c.path || inst.Link != c.link {
			t.Errorf("Installation(%s) = %+v, %v", c.executable, inst, err)
		}
	}
}

func install(t *testing.T, u *Updater) error {
	t.Helper()
	r, err := u.Latest(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := u.Installation()
	if err != nil {
		t.Fatal(err)
	}
	return u.Install(context.Background(), inst, r)
}

func assertUnchanged(t *testing.T, path, version string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != updatetest.Binary(version) {
		t.Fatalf("executable changed: %q, %v", data, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
}

func TestInstallReplacesExecutableThroughSymlink(t *testing.T) {
	gh := updatetest.New(t)
	gh.JSON(latestURL, gh.Release("1.2.0", "linux", "arm64"))
	u, path := newTestUpdater(t, gh, "1.0.0")
	if err := os.Chmod(path, 0o750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "projectory")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	u.Executable = link
	if err := install(t, u); err != nil {
		t.Fatal(err)
	}
	assertUnchanged(t, path, "1.2.0")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o750 {
		t.Fatalf("permissions not preserved: %v %v", info.Mode(), err)
	}
	if target, err := os.Readlink(link); err != nil || target != path {
		t.Fatalf("symlink changed: %q %v", target, err)
	}
	if !gh.Downloaded() {
		t.Fatal("no asset downloaded")
	}
}

func TestInstallRejectsBadReleasesAndKeepsExecutable(t *testing.T) {
	name := "projectory_1.2.0_linux_arm64.tar.gz"
	good := updatetest.File("projectory", updatetest.Binary("1.2.0"))
	cases := map[string]struct {
		archive []byte
		sums    []byte // nil means correct checksums
		want    string
	}{
		"bad checksum":    {updatetest.Archive(good), []byte(strings.Repeat("0", 64) + "  " + name + "\n"), "SHA-256"},
		"missing sum":     {updatetest.Archive(good), []byte(strings.Repeat("0", 64) + "  other.tar.gz\n"), "no incluye"},
		"duplicate sum":   {updatetest.Archive(good), nil, "no válido"},
		"traversal":       {updatetest.Archive(updatetest.File("../projectory", "x"), good), nil, "no permitida"},
		"nested path":     {updatetest.Archive(updatetest.File("bin/projectory", "x")), nil, "no permitida"},
		"absolute path":   {updatetest.Archive(updatetest.File("/projectory", "x")), nil, "no permitida"},
		"symlink":         {updatetest.Archive(updatetest.Entry{Header: tar.Header{Name: "projectory", Typeflag: tar.TypeSymlink, Linkname: "/bin/sh"}}), nil, "no permitida"},
		"duplicate entry": {updatetest.Archive(good, good), nil, "duplicada"},
		"missing binary":  {updatetest.Archive(updatetest.File("LICENSE", "MIT")), nil, "falta el ejecutable"},
		"not gzip":        {[]byte("plain text, not an archive"), nil, "no válido"},
		"truncated":       {updatetest.Archive(good)[:40], nil, "no válido"},
		"oversized entry": {updatetest.Archive(good, updatetest.Entry{Header: tar.Header{Name: "LICENSE", Typeflag: tar.TypeReg, Size: 1 << 40}, Body: "MIT"}), nil, "demasiado grande"},
		"empty binary":    {updatetest.Archive(updatetest.File("projectory", "")), nil, "vacío"},
		"wrong version":   {updatetest.Archive(updatetest.File("projectory", updatetest.Binary("6.6.6"))), nil, "no reporta la versión"},
	}
	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			gh := updatetest.New(t)
			sums := c.sums
			if sums == nil {
				sums = updatetest.Checksums(map[string][]byte{name: c.archive})
				if label == "duplicate sum" {
					sums = append(sums, sums...)
				}
			}
			gh.JSON(latestURL, gh.Publish("v1.2.0", false, map[string][]byte{name: c.archive, "checksums.txt": sums}))
			u, path := newTestUpdater(t, gh, "1.0.0")
			err := install(t, u)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
			assertUnchanged(t, path, "1.0.0")
		})
	}
}

func TestInstallRequiresExpectedAssetsAndStandalone(t *testing.T) {
	gh := updatetest.New(t)
	gh.JSON(latestURL, gh.Release("1.2.0", "darwin", "amd64"))
	u, path := newTestUpdater(t, gh, "1.0.0")
	if err := install(t, u); err == nil || !strings.Contains(err.Error(), "no publica") {
		t.Fatalf("missing platform asset: %v", err)
	}
	u.GOOS = "windows"
	if err := install(t, u); err == nil || !strings.Contains(err.Error(), "no soportada") {
		t.Fatalf("unsupported platform: %v", err)
	}
	u.GOOS, u.Version = "darwin", "0.1.0-dev"
	if err := install(t, u); err == nil {
		t.Fatal("development build replaced")
	}
	if gh.Downloaded() {
		t.Fatal("asset downloaded for rejected install")
	}
	assertUnchanged(t, path, "1.0.0")
}

func TestInstallHonoursCancellation(t *testing.T) {
	gh := updatetest.New(t)
	gh.JSON(latestURL, gh.Release("1.2.0", "linux", "arm64"))
	u, path := newTestUpdater(t, gh, "1.0.0")
	r, err := u.Latest(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	inst, _ := u.Installation()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := u.Install(ctx, inst, r); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	assertUnchanged(t, path, "1.0.0")
}

func TestRedirectsMustStayOnHTTPS(t *testing.T) {
	gh := updatetest.New(t)
	gh.Handle(latestURL, updatetest.Response{Status: http.StatusFound, Header: http.Header{"Location": {"http://api.github.com/elsewhere"}}})
	u, _ := newTestUpdater(t, gh, "1.0.0")
	if _, err := u.Latest(context.Background(), false); err == nil || !strings.Contains(err.Error(), "insegura") {
		t.Fatalf("err = %v", err)
	}
}

func TestExtractBoundsWholeUncompressedStream(t *testing.T) {
	binary := updatetest.File("projectory", updatetest.Binary("1.2.0"))
	notices := updatetest.File("THIRD_PARTY_NOTICES.md", strings.Repeat("n", 8<<10))
	if err := extractBinary(bytes.NewReader(updatetest.Archive(binary, notices)), io.Discard, 64<<10); err != nil {
		t.Fatalf("archive within limit rejected: %v", err)
	}
	// A skipped entry larger than the remaining budget.
	if err := extractBinary(bytes.NewReader(updatetest.Archive(binary, notices)), io.Discard, 6<<10); err == nil || !strings.Contains(err.Error(), "demasiado grande") {
		t.Fatalf("oversized skipped entry: %v", err)
	}
	// Valid tar followed by a large compressed tail of zeros.
	var tarball bytes.Buffer
	tw := tar.NewWriter(&tarball)
	if err := tw.WriteHeader(&binary.Header); err != nil {
		t.Fatal(err)
	}
	tw.Write([]byte(binary.Body))
	tw.Close()
	var gzipped bytes.Buffer
	gz := gzip.NewWriter(&gzipped)
	gz.Write(tarball.Bytes())
	gz.Write(make([]byte, 1<<20))
	gz.Close()
	if err := extractBinary(bytes.NewReader(gzipped.Bytes()), io.Discard, 64<<10); !errors.Is(err, errTooLarge) {
		t.Fatalf("gzip tail beyond limit: %v", err)
	}
}

func TestInstallRefusesNonNewerAndChangedTargets(t *testing.T) {
	gh := updatetest.New(t)
	gh.JSON(latestURL, gh.Release("1.2.0", "linux", "arm64"))
	u, path := newTestUpdater(t, gh, "1.2.0")
	r, err := u.Latest(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	inst, _ := u.Installation()
	if err := u.Install(context.Background(), inst, r); err == nil || !strings.Contains(err.Error(), "no es posterior") {
		t.Fatalf("same version installed: %v", err)
	}
	if gh.Downloaded() {
		t.Fatal("downloaded a non-newer release")
	}

	u, path = newTestUpdater(t, gh, "1.0.0")
	inst, _ = u.Installation()
	// Another updater replaced the executable after inspection.
	other := path + ".other"
	if err := os.WriteFile(other, []byte(updatetest.Binary("1.1.0")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(other, path); err != nil {
		t.Fatal(err)
	}
	if err := u.Install(context.Background(), inst, r); err == nil || !strings.Contains(err.Error(), "cambió") {
		t.Fatalf("changed target replaced: %v", err)
	}
	assertUnchanged(t, path, "1.1.0")
}
