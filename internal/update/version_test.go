package update

import "testing"

func TestParseVersionRejectsDevelopmentAndLooseVersions(t *testing.T) {
	for _, s := range []string{"0.1.0-dev", "dev", "", "1.0", "v1.0.0", "01.0.0", "1.0.0-rc.0", "1.0.0-beta.1", "1.0.0+meta", "1.0.0-rc.1.2", "99999999999999999999.0.0"} {
		if _, err := ParseVersion(s); err == nil {
			t.Errorf("ParseVersion(%q) accepted", s)
		}
	}
	if _, err := ParseTag("1.0.0"); err == nil {
		t.Error("tag without v prefix accepted")
	}
	v, err := ParseTag("v1.2.3-rc.4")
	if err != nil || v != (Version{1, 2, 3, 4}) || v.String() != "1.2.3-rc.4" || v.Tag() != "v1.2.3-rc.4" {
		t.Fatalf("ParseTag = %+v, %v", v, err)
	}
}

func TestCompareOrdersPrereleasesBeforeStable(t *testing.T) {
	ordered := []string{"0.9.9", "1.0.0-rc.1", "1.0.0-rc.2", "1.0.0-rc.10", "1.0.0", "1.0.1-rc.1", "1.0.1", "1.10.0", "2.0.0"}
	for i := range ordered {
		for j := range ordered {
			a, _ := ParseVersion(ordered[i])
			b, _ := ParseVersion(ordered[j])
			want := 0
			if i < j {
				want = -1
			} else if i > j {
				want = 1
			}
			if got := a.Compare(b); got != want {
				t.Errorf("Compare(%s, %s) = %d, want %d", a, b, got, want)
			}
		}
	}
}
