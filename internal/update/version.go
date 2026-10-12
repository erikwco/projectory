package update

import (
	"cmp"
	"fmt"
	"regexp"
	"strconv"
)

// Version is a release version in the only shape Projectory publishes:
// MAJOR.MINOR.PATCH with an optional -rc.N pre-release suffix.
type Version struct {
	Major, Minor, Patch int
	RC                  int // 0 means a stable release
}

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-rc\.([1-9][0-9]*))?$`)

// ParseVersion parses a version without the "v" tag prefix. Development
// builds such as "0.1.0-dev" are deliberately rejected.
func ParseVersion(s string) (Version, error) {
	match := versionPattern.FindStringSubmatch(s)
	if match == nil {
		return Version{}, fmt.Errorf("versión no soportada: %q", s)
	}
	var parts [4]int
	for i, part := range match[1:] {
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, fmt.Errorf("versión no soportada: %q", s)
		}
		parts[i] = n
	}
	return Version{parts[0], parts[1], parts[2], parts[3]}, nil
}

// ParseTag parses a release tag such as "v1.2.0" or "v1.2.0-rc.1".
func ParseTag(tag string) (Version, error) {
	if len(tag) < 2 || tag[0] != 'v' {
		return Version{}, fmt.Errorf("etiqueta no soportada: %q", tag)
	}
	return ParseVersion(tag[1:])
}

func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.RC > 0 {
		s += fmt.Sprintf("-rc.%d", v.RC)
	}
	return s
}

func (v Version) Tag() string { return "v" + v.String() }

func (v Version) Prerelease() bool { return v.RC > 0 }

// Compare returns -1, 0 or +1. A release candidate sorts before the stable
// release with the same MAJOR.MINOR.PATCH.
func (v Version) Compare(other Version) int {
	if c := cmp.Compare(v.Major, other.Major); c != 0 {
		return c
	}
	if c := cmp.Compare(v.Minor, other.Minor); c != 0 {
		return c
	}
	if c := cmp.Compare(v.Patch, other.Patch); c != 0 {
		return c
	}
	switch {
	case v.RC == other.RC:
		return 0
	case v.RC == 0:
		return 1
	case other.RC == 0:
		return -1
	}
	return cmp.Compare(v.RC, other.RC)
}
