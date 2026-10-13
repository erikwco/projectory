package catalog

import (
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Exact returns the locations a reference names without guessing. An ID or a
// path names one location; a project name or proyecto/etiqueta, compared
// without case, can name several, which callers must not choose implicitly.
func Exact(locations []Location, ref string) []Location {
	path, _ := NormalizePath(ref, false)
	for _, l := range locations {
		if string(l.ID) == ref || l.Path == path {
			return []Location{l}
		}
	}
	var matches []Location
	for _, l := range locations {
		if strings.EqualFold(l.Project.Name, ref) || strings.EqualFold(l.Project.Name+"/"+l.Label, ref) {
			matches = append(matches, l)
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return lessLocation(matches[i], matches[j]) })
	return matches
}

// Navigation is the outcome of resolving a reference typed to change folder.
// Exact results may still hold several locations of the same name; approximate
// results always need the user's confirmation.
type Navigation struct {
	Exact     bool
	Locations []Location
}

// Navigate prefers exact references and only then ranks approximate names.
// Administrative commands keep using Exact so they never act on a guess.
func Navigate(locations []Location, ref string) Navigation {
	if exact := Exact(locations, ref); len(exact) > 0 {
		return Navigation{Exact: true, Locations: exact}
	}
	return Navigation{Locations: Approximate(locations, ref)}
}

// LocationRank orders the locations of a project: the main copy, other
// folders, then worktrees.
func LocationRank(l Location) int {
	if l.Kind == Worktree {
		return 2
	}
	if l.Label == "principal" {
		return 0
	}
	return 1
}

func lessLocation(a, b Location) bool {
	if an, bn := strings.ToLower(a.Project.Name), strings.ToLower(b.Project.Name); an != bn {
		return an < bn
	}
	if a.Project.ID != b.Project.ID {
		return a.Project.ID < b.Project.ID
	}
	if LocationRank(a) != LocationRank(b) {
		return LocationRank(a) < LocationRank(b)
	}
	return a.Path < b.Path
}

// Tiers, from closest to farthest. Project names come first; labels and folder
// names only help when the name does not match; spelling mistakes come last.
const (
	nameEqual = iota
	namePrefix
	nameWords
	nameSubstring
	otherEqual
	otherPrefix
	otherWords
	otherSubstring
	nameTypo
	otherTypo
)

type score struct{ tier, distance, position, length int }

func (a score) less(b score) bool {
	if a.tier != b.tier {
		return a.tier < b.tier
	}
	if a.distance != b.distance {
		return a.distance < b.distance
	}
	if a.position != b.position {
		return a.position < b.position
	}
	return a.length < b.length
}

type folded struct {
	compact string
	words   []string
}

// fold ignores case, spaces, and punctuation, so "mi-tienda" and "Mi Tienda"
// compare as the same name.
func fold(text string) folded {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	return folded{compact: strings.Join(words, ""), words: words}
}

// Approximate ranks every location whose project name, label, or folder name
// resembles the query. All locations of a matching project are kept. The order
// depends only on match quality and stable identifiers, never on visits.
func Approximate(locations []Location, query string) []Location {
	q := fold(query)
	if q.compact == "" {
		return nil
	}
	type candidate struct {
		Location
		score
	}
	var found []candidate
	for _, l := range locations {
		best, ok := closeness(q, fold(l.Project.Name), 0)
		for _, other := range []string{l.Label, filepath.Base(l.Path)} {
			if s, matched := closeness(q, fold(other), otherEqual); matched && (!ok || s.less(best)) {
				best, ok = s, true
			}
		}
		if ok {
			best.length = utf8.RuneCountInString(fold(l.Project.Name).compact)
			found = append(found, candidate{l, best})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].score != found[j].score {
			return found[i].score.less(found[j].score)
		}
		return lessLocation(found[i].Location, found[j].Location)
	})
	result := make([]Location, len(found))
	for i, c := range found {
		result[i] = c.Location
	}
	return result
}

// closeness scores one field. offset selects the name or the secondary tiers.
func closeness(q, v folded, offset int) (score, bool) {
	if v.compact == "" {
		return score{}, false
	}
	if v.compact == q.compact {
		return score{tier: offset + nameEqual}, true
	}
	if strings.HasPrefix(v.compact, q.compact) {
		return score{tier: offset + namePrefix}, true
	}
	if position, ok := wordPrefixes(q.words, v.words); ok {
		return score{tier: offset + nameWords, position: position}, true
	}
	if position := strings.Index(v.compact, q.compact); position >= 0 {
		return score{tier: offset + nameSubstring, position: position}, true
	}
	allowed := typoAllowance(q.compact)
	if allowed == 0 {
		return score{}, false
	}
	distance := allowed + 1
	for _, word := range append([]string{v.compact}, v.words...) {
		distance = min(distance, typoDistance(q.compact, word))
	}
	if distance > allowed {
		return score{}, false
	}
	tier := nameTypo
	if offset != 0 {
		tier = otherTypo
	}
	return score{tier: tier, distance: distance}, true
}

// wordPrefixes reports whether each query word starts a distinct word of the
// field, in order, and returns the index of the first such word.
func wordPrefixes(query, words []string) (int, bool) {
	first, next := -1, 0
	for _, q := range query {
		for next < len(words) && !strings.HasPrefix(words[next], q) {
			next++
		}
		if next == len(words) {
			return 0, false
		}
		if first < 0 {
			first = next
		}
		next++
	}
	return first, first >= 0
}

// Resembles reports whether word is name with a small typing mistake, within
// the same bounds that approximate navigation tolerates.
func Resembles(word, name string) bool {
	allowed := typoAllowance(word)
	return allowed > 0 && editDistance([]rune(strings.ToLower(word)), []rune(strings.ToLower(name))) <= allowed
}

// typoAllowance bounds spelling tolerance by query length: short queries would
// otherwise resemble almost every name.
func typoAllowance(query string) int {
	switch n := utf8.RuneCountInString(query); {
	case n < 3:
		return 0
	case n <= 5:
		return 1
	default:
		return 2
	}
}

// typoDistance compares the query with the whole word and with its beginning,
// so a misspelled prefix such as "blgo" still finds "blog-admin".
func typoDistance(query, word string) int {
	q, w := []rune(query), []rune(word)
	best := editDistance(q, w)
	for n := len(q) - 1; n <= len(q)+1; n++ {
		if n > 0 && n < len(w) {
			best = min(best, editDistance(q, w[:n]))
		}
	}
	return best
}

// editDistance counts insertions, deletions, substitutions, and transpositions
// of adjacent characters (optimal string alignment).
func editDistance(a, b []rune) int {
	rows := make([][]int, len(a)+1)
	for i := range rows {
		rows[i] = make([]int, len(b)+1)
		rows[i][0] = i
	}
	for j := range rows[0] {
		rows[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			rows[i][j] = min(rows[i-1][j]+1, rows[i][j-1]+1, rows[i-1][j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				rows[i][j] = min(rows[i][j], rows[i-2][j-2]+1)
			}
		}
	}
	return rows[len(a)][len(b)]
}
