package catalog

import (
	"slices"
	"testing"
)

func location(id, project, name, label, path string, kind Kind) Location {
	return Location{ID: LocationID(id), Project: Project{ID: ProjectID(project), Name: name}, Label: label, Path: path, Kind: kind, Available: true}
}

func ids(locations []Location) []string {
	result := make([]string, len(locations))
	for i, l := range locations {
		result[i] = string(l.ID)
	}
	return result
}

func fixture() []Location {
	return []Location{
		location("blog-wt", "p-blog", "Blog", "", "/dev/blog-feature", Worktree),
		location("blog-main", "p-blog", "Blog", "principal", "/dev/blog", Repository),
		location("adm", "p-admin", "Blog Admin", "", "/dev/blog-admin", Repository),
		location("shop", "p-shop", "Mi Tienda", "principal", "/dev/mi tienda", Directory),
		location("old-shop", "p-old", "Tienda Vieja", "copia 2025", "/archive/shop", Directory),
		location("api", "p-api", "Pagos", "principal", "/dev/payments-api", Repository),
	}
}

func TestExactKeepsStrictResolution(t *testing.T) {
	all := fixture()
	for ref, want := range map[string][]string{
		"shop":           {"shop"},
		"/dev/blog":      {"blog-main"},
		"mi tienda":      {"shop"},
		"MI TIENDA":      {"shop"},
		"Blog/principal": {"blog-main"},
		"Blog":           {"blog-main", "blog-wt"},
		"tienda":         nil,
		"blgo":           nil,
		"Mi Tiend":       nil,
	} {
		if got := ids(Exact(all, ref)); !slices.Equal(got, want) && (len(got) != 0 || len(want) != 0) {
			t.Errorf("Exact(%q) = %v, want %v", ref, got, want)
		}
	}
}

func TestNavigatePrefersExactNames(t *testing.T) {
	found := Navigate(fixture(), "blog")
	if !found.Exact || !slices.Equal(ids(found.Locations), []string{"blog-main", "blog-wt"}) {
		t.Fatalf("exact name with several locations must offer all of them: %+v", found)
	}
	found = Navigate(fixture(), "Mi Tienda")
	if !found.Exact || !slices.Equal(ids(found.Locations), []string{"shop"}) {
		t.Fatalf("exact unique name: %+v", found)
	}
}

func TestApproximateRanking(t *testing.T) {
	all := fixture()
	for _, tc := range []struct {
		query string
		want  []string
	}{
		// A name prefix beats a later word; every location of the project stays.
		{"tien", []string{"old-shop", "shop"}},
		{"blo", []string{"blog-main", "blog-wt", "adm"}},
		{"admin", []string{"adm"}},
		// Word prefixes in order.
		{"blog adm", []string{"adm"}},
		// Substrings, earliest first.
		{"enda", []string{"old-shop", "shop"}},
		// Labels and folder names help only after names.
		{"copia", []string{"old-shop"}},
		{"payments", []string{"api"}},
		// Bounded spelling mistakes: transposition, omission, and a misspelled prefix.
		{"blgo", []string{"blog-main", "blog-wt", "adm"}},
		{"tinda", []string{"shop", "old-shop"}},
		{"pagso", []string{"api"}},
		// Punctuation and case are ignored, but approximate matches never become
		// exact; longer queries tolerate two mistakes ("mi" before "tienda").
		{"MI-TIENDA", []string{"shop", "old-shop"}},
		// Unrelated text and short queries never invent matches.
		{"zzzz", nil},
		{"xq", nil},
		{"   ", nil},
	} {
		found := Navigate(all, tc.query)
		if found.Exact {
			t.Errorf("%q must not resolve exactly", tc.query)
		}
		if got := ids(found.Locations); !slices.Equal(got, tc.want) && (len(got) != 0 || len(tc.want) != 0) {
			t.Errorf("Approximate(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestApproximateIsDeterministicForTies(t *testing.T) {
	a := location("a", "p1", "Notas", "", "/b/notas", Directory)
	b := location("b", "p2", "notas", "", "/a/notas", Directory)
	c := location("c", "p2", "notas", "principal", "/c/notas", Directory)
	want := []string{"a", "c", "b"}
	for _, order := range [][]Location{{a, b, c}, {c, b, a}, {b, a, c}} {
		if got := ids(Approximate(order, "nota")); !slices.Equal(got, want) {
			t.Fatalf("tied matches depend on input order: %v", got)
		}
	}
}

func TestApproximateKeepsUnavailableAndIgnoresVisits(t *testing.T) {
	missing := location("missing", "p1", "Tienda", "", "/gone/tienda", Directory)
	missing.Available = false
	popular := location("popular", "p2", "Mi Tiendita", "", "/dev/tiendita", Directory)
	popular.Visits = 500
	if got := ids(Approximate([]Location{popular, missing}, "tiend")); !slices.Equal(got, []string{"missing", "popular"}) {
		t.Fatalf("visits overrode closeness or an unavailable location vanished: %v", got)
	}
}

func TestEditDistance(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{{"blgo", "blog", 1}, {"tienda", "tienda", 0}, {"tinda", "tienda", 1}, {"abc", "", 3}, {"kitten", "sitting", 3}} {
		if got := editDistance([]rune(tc.a), []rune(tc.b)); got != tc.want {
			t.Errorf("editDistance(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestResemblesUsesBoundedTypos(t *testing.T) {
	for _, tc := range []struct {
		word, name string
		want       bool
	}{{"lsit", "list", true}, {"dahs", "dash", true}, {"lsit", "edit", false}, {"lsit", "init", false}, {"ls", "list", false}, {"list", "list", true}} {
		if got := Resembles(tc.word, tc.name); got != tc.want {
			t.Errorf("Resembles(%q, %q) = %v", tc.word, tc.name, got)
		}
	}
}
