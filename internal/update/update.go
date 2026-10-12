// Package update checks Projectory's official GitHub releases and replaces a
// standalone executable with a verified release binary. It never touches the
// project catalog or user settings.
package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	repository   = "erikwco/projectory"
	apiBase      = "https://api.github.com/repos/" + repository
	downloadBase = "https://github.com/" + repository + "/releases/download"
	apiVersion   = "2026-03-10"

	// BrewUpgrade is the only supported way to update a Homebrew installation.
	BrewUpgrade = "brew update && brew upgrade erikwco/tap/projectory"

	checksumsAsset = "checksums.txt"
	apiTimeout     = 30 * time.Second
	apiLimit       = 4 << 20
)

// Updater talks to the fixed official release origins. Tests may replace the
// executable path, platform and HTTP transport, never the origins.
type Updater struct {
	Version      string
	Executable   string // empty means os.Executable
	GOOS, GOARCH string
	Transport    http.RoundTripper // nil means a clone of http.DefaultTransport
}

func New(version string) *Updater {
	return &Updater{Version: version, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
}

type Method int

const (
	Standalone Method = iota
	Homebrew
	Development
)

func (m Method) String() string {
	switch m {
	case Homebrew:
		return "Homebrew"
	case Development:
		return "compilación de desarrollo"
	}
	return "binario independiente"
}

// Installation describes the running executable. Path is the regular file an
// update replaces; Link is the invoked symlink, kept untouched, if any.
type Installation struct {
	Method  Method
	Path    string
	Link    string
	Current Version     // zero for development builds
	file    os.FileInfo // Path as inspected, to detect concurrent replacement
}

func (u *Updater) Installation() (Installation, error) {
	executable := u.Executable
	if executable == "" {
		var err error
		if executable, err = os.Executable(); err != nil {
			return Installation{}, err
		}
	}
	executable, err := filepath.Abs(executable)
	if err != nil {
		return Installation{}, err
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return Installation{}, err
	}
	file, err := os.Stat(resolved)
	if err != nil {
		return Installation{}, err
	}
	inst := Installation{Path: resolved, file: file}
	if resolved != executable {
		inst.Link = executable
	}
	marker := string(filepath.Separator) + "Cellar" + string(filepath.Separator) + "projectory" + string(filepath.Separator)
	// A keg receipt is extra evidence; when in doubt, refuse to touch the file.
	receipt := filepath.Join(filepath.Dir(filepath.Dir(resolved)), "INSTALL_RECEIPT.json")
	if _, err := os.Stat(receipt); strings.Contains(resolved, marker) || err == nil {
		inst.Method = Homebrew
	}
	if inst.Current, err = ParseVersion(u.Version); err != nil && inst.Method != Homebrew {
		inst.Method = Development
	}
	return inst, nil
}

// Release is a published release with the assets it advertises.
type Release struct {
	Version Version
	Assets  map[string]int64 // name -> size in bytes
}

type githubRelease struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

func (r githubRelease) release() (Release, error) {
	v, err := ParseTag(r.TagName)
	if err != nil {
		return Release{}, err
	}
	assets := make(map[string]int64, len(r.Assets))
	for _, a := range r.Assets {
		assets[a.Name] = a.Size
	}
	return Release{Version: v, Assets: assets}, nil
}

// ErrNoRelease means GitHub has no published release for the chosen channel.
var ErrNoRelease = errors.New("no hay versiones publicadas")

// Latest returns the newest stable release, or with prerelease the newest
// release of any kind among the first page of non-draft releases.
func (u *Updater) Latest(ctx context.Context, prerelease bool) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	if !prerelease {
		var latest githubRelease
		if err := u.getJSON(ctx, apiBase+"/releases/latest", &latest); err != nil {
			return Release{}, err
		}
		r, err := latest.release()
		if err == nil && (latest.Draft || latest.Prerelease || r.Version.Prerelease()) {
			err = fmt.Errorf("GitHub devolvió %q como versión estable", latest.TagName)
		}
		return r, err
	}
	var releases []githubRelease
	if err := u.getJSON(ctx, apiBase+"/releases?per_page=100", &releases); err != nil {
		return Release{}, err
	}
	var newest *Release
	for _, gr := range releases {
		r, err := gr.release()
		if gr.Draft || err != nil {
			continue
		}
		if newest == nil || r.Version.Compare(newest.Version) > 0 {
			newest = &r
		}
	}
	if newest == nil {
		return Release{}, ErrNoRelease
	}
	return *newest, nil
}

func (u *Updater) getJSON(ctx context.Context, url string, v any) error {
	var body bytes.Buffer
	if err := u.get(ctx, url, apiLimit, &body); err != nil {
		return err
	}
	if err := json.Unmarshal(body.Bytes(), v); err != nil {
		return fmt.Errorf("respuesta de GitHub no válida: %w", err)
	}
	return nil
}

func (u *Updater) client() *http.Client {
	transport := u.Transport
	if transport == nil {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.ResponseHeaderTimeout = apiTimeout
		transport = t
	}
	return &http.Client{Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("demasiadas redirecciones")
		}
		if req.URL.Scheme != "https" {
			return fmt.Errorf("redirección insegura a %s", req.URL.Redacted())
		}
		return nil
	}}
}

// get streams at most limit bytes of a successful response into w.
func (u *Updater) get(ctx context.Context, url string, limit int64, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "projectory/"+u.Version)
	if strings.HasPrefix(url, apiBase) {
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", apiVersion)
	}
	resp, err := u.client().Do(req)
	if err != nil {
		return fmt.Errorf("no se pudo contactar GitHub: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusNotFound && strings.HasPrefix(url, apiBase):
		return ErrNoRelease
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return rateLimitError(resp.Header)
	default:
		return fmt.Errorf("GitHub respondió %s para %s", resp.Status, url)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return fmt.Errorf("descarga interrumpida: %w", err)
	}
	if n > limit {
		return fmt.Errorf("respuesta demasiado grande desde %s", url)
	}
	return nil
}

func rateLimitError(h http.Header) error {
	msg := "GitHub limitó las solicitudes anónimas; inténtalo más tarde"
	if reset, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		msg += " (se restablece " + time.Unix(reset, 0).Local().Format("15:04") + ")"
	} else if after := h.Get("Retry-After"); after != "" {
		msg += " (reintenta en " + after + " s)"
	}
	return errors.New(msg)
}
