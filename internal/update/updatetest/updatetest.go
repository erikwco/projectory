// Package updatetest fakes the public GitHub releases contract for updater
// tests. Requests keep their official URLs; only the transport is redirected
// to a local server.
package updatetest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
)

const (
	API       = "https://api.github.com/repos/erikwco/projectory"
	Downloads = "https://github.com/erikwco/projectory/releases/download"
	assetHost = "https://release-assets.githubusercontent.com/projectory"
)

type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// GitHub serves canned responses keyed by full official URL.
type GitHub struct {
	server  *httptest.Server
	mu      sync.Mutex
	routes  map[string]Response
	seen    []string
	headers map[string]http.Header
}

func New(t testing.TB) *GitHub {
	g := &GitHub{routes: map[string]Response{}, headers: map[string]http.Header{}}
	g.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := "https://" + r.Host + r.URL.RequestURI()
		g.mu.Lock()
		g.seen = append(g.seen, url)
		g.headers[url] = r.Header.Clone()
		resp, ok := g.routes[url]
		g.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.Status)
		w.Write(resp.Body)
	}))
	t.Cleanup(g.server.Close)
	return g
}

// Transport sends every request to the fake server, keeping its Host.
func (g *GitHub) Transport() http.RoundTripper {
	return roundTripper(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.Host = r.URL.Host
		r.URL.Scheme, r.URL.Host = "http", g.server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func (g *GitHub) Handle(url string, resp Response) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.routes[url] = resp
}

func (g *GitHub) JSON(url string, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	g.Handle(url, Response{Status: http.StatusOK, Body: body})
}

// Requests returns every URL requested so far.
func (g *GitHub) Requests() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.seen...)
}

// Header returns the headers of the last request to url.
func (g *GitHub) Header(url string) http.Header {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.headers[url]
}

// Downloaded reports whether any release asset was requested.
func (g *GitHub) Downloaded() bool {
	for _, url := range g.Requests() {
		if strings.HasPrefix(url, Downloads) || strings.HasPrefix(url, assetHost) {
			return true
		}
	}
	return false
}

// Publish serves assets the way GitHub does, through a redirect to its asset
// host, and returns the release object as the REST API describes it.
func (g *GitHub) Publish(tag string, prerelease bool, assets map[string][]byte) map[string]any {
	names := make([]string, 0, len(assets))
	for name := range assets {
		names = append(names, name)
	}
	sort.Strings(names)
	list := []map[string]any{}
	for _, name := range names {
		official := Downloads + "/" + tag + "/" + name
		cdn := assetHost + "/" + tag + "/" + name
		g.Handle(official, Response{Status: http.StatusFound, Header: http.Header{"Location": {cdn}}})
		g.Handle(cdn, Response{Status: http.StatusOK, Body: assets[name]})
		list = append(list, map[string]any{"name": name, "browser_download_url": official, "size": len(assets[name])})
	}
	return map[string]any{"tag_name": tag, "draft": false, "prerelease": prerelease, "assets": list}
}

// Entry is one tar member; Body is ignored for non-regular entries. A Body
// shorter than Header.Size yields a truncated archive.
type Entry struct {
	Header tar.Header
	Body   string
}

func File(name, body string) Entry {
	return Entry{Header: tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len(body))}, Body: body}
}

// Binary is a stand-in executable that answers --version like Cobra does.
func Binary(version string) string {
	return "#!/bin/sh\necho 'projectory version " + version + "'\n"
}

func Archive(entries ...Entry) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	truncated := false
	for _, e := range entries {
		truncated = truncated || (e.Header.Typeflag == tar.TypeReg && e.Header.Size > int64(len(e.Body)))
		if err := tw.WriteHeader(&e.Header); err != nil {
			panic(err)
		}
		if e.Header.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(e.Body)); err != nil {
				panic(err)
			}
		}
	}
	if err := tw.Close(); err != nil && !truncated {
		panic(err)
	}
	if err := gz.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func Checksums(files map[string][]byte) []byte {
	var buf bytes.Buffer
	for name, data := range files {
		sum := sha256.Sum256(data)
		fmt.Fprintf(&buf, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	return buf.Bytes()
}

// Release builds a complete, valid release for one platform and serves it.
func (g *GitHub) Release(version, goos, goarch string) map[string]any {
	name := fmt.Sprintf("projectory_%s_%s_%s.tar.gz", version, goos, goarch)
	archive := Archive(File("projectory", Binary(version)), File("LICENSE", "MIT"), File("README.md", "# Projectory"), File("THIRD_PARTY_NOTICES.md", "notices"))
	return g.Publish("v"+version, strings.Contains(version, "-"), map[string][]byte{
		name:            archive,
		"checksums.txt": Checksums(map[string][]byte{name: archive}),
	})
}
