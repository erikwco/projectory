package update

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	binaryName       = "projectory"
	checksumsLimit   = 64 << 10
	archiveLimit     = 256 << 20 // compressed download and uncompressed contents
	downloadTimeout  = 10 * time.Minute
	validateTimeout  = 10 * time.Second
	tempBinaryPrefix = ".projectory-update-"
)

// archiveFiles are the only entries a release archive may contain.
var archiveFiles = map[string]bool{binaryName: true, "LICENSE": true, "README.md": true, "THIRD_PARTY_NOTICES.md": true}

// ArchiveName is the release asset for a version and platform.
func ArchiveName(v Version, goos, goarch string) string {
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", binaryName, v, goos, goarch)
}

func supported(goos, goarch string) bool {
	return (goos == "darwin" || goos == "linux") && (goarch == "amd64" || goarch == "arm64")
}

// Install downloads r for the updater's platform, verifies its SHA-256
// checksum before extraction, checks the candidate reports the expected
// version and atomically replaces inst.Path. On any error the current
// executable is left untouched.
func (u *Updater) Install(ctx context.Context, inst Installation, r Release) error {
	if inst.Method != Standalone {
		return fmt.Errorf("instalación %s: no se reemplaza automáticamente", inst.Method)
	}
	if r.Version.Compare(inst.Current) <= 0 {
		return fmt.Errorf("la versión %s no es posterior a la instalada (%s)", r.Version, inst.Current)
	}
	if !supported(u.GOOS, u.GOARCH) {
		return fmt.Errorf("plataforma no soportada: %s/%s", u.GOOS, u.GOARCH)
	}
	name := ArchiveName(r.Version, u.GOOS, u.GOARCH)
	size, ok := r.Assets[name]
	if _, sums := r.Assets[checksumsAsset]; !ok || !sums || size <= 0 || size > archiveLimit {
		return fmt.Errorf("la versión %s no publica %s y %s válidos", r.Version, name, checksumsAsset)
	}
	if err := inst.unchanged(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	// Create the candidate first so an unwritable directory fails before any download.
	candidate, err := os.CreateTemp(filepath.Dir(inst.Path), tempBinaryPrefix+"*")
	if err != nil {
		return fmt.Errorf("no se puede escribir junto a %s: %w", inst.Path, err)
	}
	defer os.Remove(candidate.Name())
	defer candidate.Close()

	base := downloadBase + "/" + r.Version.Tag() + "/"
	var sums bytes.Buffer
	if err := u.get(ctx, base+checksumsAsset, checksumsLimit, &sums); err != nil {
		return err
	}
	want, err := checksumFor(sums.Bytes(), name)
	if err != nil {
		return err
	}

	archive, err := os.CreateTemp("", tempBinaryPrefix+"*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(archive.Name())
	defer archive.Close()
	hash := sha256.New()
	if err := u.get(ctx, base+name, size, io.MultiWriter(archive, hash)); err != nil {
		return err
	}
	if !bytes.Equal(hash.Sum(nil), want) {
		return fmt.Errorf("la suma SHA-256 de %s no coincide; no se instaló nada", name)
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := extractBinary(archive, candidate, archiveLimit); err != nil {
		return fmt.Errorf("archivo %s no válido: %w", name, err)
	}
	if err := candidate.Chmod(inst.file.Mode().Perm()); err != nil {
		return err
	}
	if err := candidate.Sync(); err != nil {
		return err
	}
	if err := candidate.Close(); err != nil {
		return err
	}
	if err := validate(ctx, candidate.Name(), r.Version); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Best effort against a concurrent update: the window until rename is tiny.
	if err := inst.unchanged(); err != nil {
		return err
	}
	if err := os.Rename(candidate.Name(), inst.Path); err != nil {
		return err
	}
	if dir, err := os.Open(filepath.Dir(inst.Path)); err == nil {
		_ = dir.Sync()
		dir.Close()
	}
	return nil
}

// unchanged reports whether Path is still the file Installation inspected.
func (inst Installation) unchanged() error {
	now, err := os.Stat(inst.Path)
	if err != nil {
		return err
	}
	if inst.file == nil || !os.SameFile(inst.file, now) || inst.file.Size() != now.Size() || !inst.file.ModTime().Equal(now.ModTime()) {
		return fmt.Errorf("%s cambió durante la actualización; no se reemplazó", inst.Path)
	}
	return nil
}

// checksumFor finds exactly one "<sha256>  <name>" line for name.
func checksumFor(manifest []byte, name string) ([]byte, error) {
	var found []byte
	scanner := bufio.NewScanner(bytes.NewReader(manifest))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 {
			return nil, fmt.Errorf("%s no válido", checksumsAsset)
		}
		if strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		sum, err := hex.DecodeString(fields[0])
		if err != nil || len(sum) != sha256.Size || found != nil {
			return nil, fmt.Errorf("%s no válido para %s", checksumsAsset, name)
		}
		found = sum
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if found == nil {
		return nil, fmt.Errorf("%s no incluye %s", checksumsAsset, name)
	}
	return found, nil
}

// extractBinary copies the executable entry into dst after rejecting any
// unexpected, duplicate, nested or non-regular entry. limit bounds the whole
// uncompressed stream, including skipped entries and trailing data.
func extractBinary(src io.Reader, dst io.Writer, limit int64) error {
	gz, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer gz.Close()
	stream := &boundedReader{r: gz, left: limit}
	tr := tar.NewReader(stream)
	seen := map[string]bool{}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if !archiveFiles[h.Name] || h.Typeflag != tar.TypeReg {
			return fmt.Errorf("entrada no permitida: %q", h.Name)
		}
		if seen[h.Name] {
			return fmt.Errorf("entrada duplicada: %q", h.Name)
		}
		seen[h.Name] = true
		if h.Size < 0 || h.Size > stream.left {
			return fmt.Errorf("entrada demasiado grande: %q", h.Name)
		}
		if h.Name != binaryName {
			continue
		}
		if h.Size == 0 {
			return fmt.Errorf("el ejecutable %s está vacío", binaryName)
		}
		if _, err := io.CopyN(dst, tr, h.Size); err != nil {
			return err
		}
	}
	if !seen[binaryName] {
		return fmt.Errorf("falta el ejecutable %s", binaryName)
	}
	// Drain trailing data so a truncated or corrupt gzip stream is reported.
	_, err = io.Copy(io.Discard, stream)
	return err
}

var errTooLarge = errors.New("contenido descomprimido demasiado grande")

// boundedReader fails, instead of silently truncating, past its limit.
type boundedReader struct {
	r    io.Reader
	left int64
}

func (b *boundedReader) Read(p []byte) (int, error) {
	if b.left <= 0 {
		// Probe for one more byte to tell an exact fit from an overflow.
		var one [1]byte
		n, err := b.r.Read(one[:])
		if n > 0 {
			return 0, errTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > b.left {
		p = p[:b.left]
	}
	n, err := b.r.Read(p)
	b.left -= int64(n)
	return n, err
}

// validate runs the candidate and requires it to report the expected version.
func validate(ctx context.Context, path string, v Version) error {
	ctx, cancel := context.WithTimeout(ctx, validateTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if got := strings.TrimSpace(string(out)); err != nil || got != binaryName+" version "+v.String() {
		return fmt.Errorf("el ejecutable descargado no reporta la versión %s; no se instaló nada", v)
	}
	return nil
}
