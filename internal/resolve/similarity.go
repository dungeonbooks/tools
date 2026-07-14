package resolve

import (
	"strings"
	"unicode"
)

// nearIdentical is the character-similarity above which two titles are treated
// as the same string written slightly differently — a typo, a dropped article.
const nearIdentical = 0.90

// subtitleSeps mark where a subtitle or alternate title begins. Cataloguers use
// them consistently, and that consistency is load-bearing here: it is what makes
// "Sapiens: A Brief History of Humankind" recognisable as Sapiens while leaving
// "Dune Messiah" a different book from "Dune".
var subtitleSeps = []string{":", " — ", " - ", ", or "}

// mainTitle drops the subtitle, if there is one.
func mainTitle(s string) string {
	cut := len(s)
	for _, sep := range subtitleSeps {
		if i := strings.Index(s, sep); i > 0 && i < cut {
			cut = i
		}
	}
	return s[:cut]
}

// normalize lowercases and collapses everything that isn't alphanumeric into
// single spaces, so punctuation, casing, and spacing differences between a
// requested title and a provider's title fall away before scoring.
func normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := true // leading space is suppressed
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(r)
			space = false
		case !space:
			b.WriteRune(' ')
			space = true
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// titleScore reports how well got covers want, on a 0-to-1 scale. Both are
// scored whole and with any subtitle removed, and the better reading wins, so a
// request for "Playing at the World" is satisfied by "Playing at the World: A
// History of Simulating Wars" without a request for "Dune" being satisfied by
// "Dune Messiah".
func titleScore(want, got string) float64 {
	w, g := strings.ToLower(want), strings.ToLower(got)
	return max(pairScore(w, g), pairScore(mainTitle(w), mainTitle(g)))
}

func pairScore(want, got string) float64 {
	w, g := normalize(want), normalize(got)
	if w == "" || g == "" {
		return 0
	}
	score := tokenOverlap(w, g)
	// The character ratio only rescues a near-identical string. It must never
	// rescue a bare prefix: "Foundation" is a 0.65 character-match for
	// "Foundation and Empire", which is a different book. Extra tokens in the
	// result — once a subtitle is off the table — mean a different work.
	if r := ratio(w, g); r >= nearIdentical && r > score {
		score = r
	}
	return score
}

// tokenOverlap scores shared words as the lesser of recall and precision, so a
// short title that merely appears inside a longer one ("It" in "It Ends With
// Us") cannot score well on containment alone. Cataloguing noise is discounted
// first: Hardcover files Jon Peterson's book as "Playing at the World, 2E,
// Volume 1", and those three trailing tokens are not what makes it a different
// book, whereas the "Messiah" in "Dune Messiah" is.
func tokenOverlap(w, g string) float64 {
	wt, gt := fieldSet(w), fieldSet(g)
	shared := make(map[string]struct{})
	for t := range wt {
		if _, ok := gt[t]; ok {
			shared[t] = struct{}{}
		}
	}
	overlap := float64(len(shared))
	recall := overlap / float64(len(pruneNoise(wt, shared)))
	precision := overlap / float64(len(pruneNoise(gt, shared)))
	return min(recall, precision)
}

// pruneNoise drops edition and volume markers, unless the request asked for them
// — a token both sides share is signal by definition. It never empties a set: a
// title that is nothing but noise is scored as it stands.
func pruneNoise(set, shared map[string]struct{}) map[string]struct{} {
	kept := make(map[string]struct{}, len(set))
	for t := range set {
		if _, ok := shared[t]; ok || !isEditionNoise(t) {
			kept[t] = struct{}{}
		}
	}
	if len(kept) == 0 {
		return set
	}
	return kept
}

// editionWords are cataloguing markers, never the substance of a title. Words
// that could carry meaning ("new", "complete", "second") are deliberately absent:
// discounting one of those could let a genuinely different book through, and a
// false positive here means handing back a real ISBN for the wrong book.
var editionWords = map[string]struct{}{
	"volume": {}, "volumes": {}, "vol": {}, "edition": {}, "editions": {}, "ed": {},
	"revised": {}, "updated": {}, "expanded": {}, "illustrated": {}, "annotated": {},
	"anniversary": {}, "reprint": {}, "unabridged": {}, "abridged": {}, "deluxe": {},
}

// isEditionNoise matches those markers plus a bare number ("Volume 1") and an
// edition ordinal ("2e", "2nd", "3rd").
func isEditionNoise(t string) bool {
	if _, ok := editionWords[t]; ok {
		return true
	}
	digits := strings.TrimRight(t, "stndrdhe")
	if digits == "" {
		return false
	}
	for _, r := range digits {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return len(digits) == len(t) || isOrdinalSuffix(t[len(digits):])
}

func isOrdinalSuffix(s string) bool {
	switch s {
	case "st", "nd", "rd", "th", "e":
		return true
	}
	return false
}

func fieldSet(s string) map[string]struct{} {
	f := strings.Fields(s)
	set := make(map[string]struct{}, len(f))
	for _, t := range f {
		set[t] = struct{}{}
	}
	return set
}

// ratio is difflib.SequenceMatcher.ratio: twice the number of runes matched by
// the Ratcliff-Obershelp decomposition, over the combined length. Python's
// autojunk heuristic only engages at 200+ elements, so titles never trip it.
func ratio(a, b string) float64 {
	ar, br := []rune(a), []rune(b)
	total := len(ar) + len(br)
	if total == 0 {
		return 1
	}
	return 2 * float64(matched(ar, br)) / float64(total)
}

// matched recursively sums the longest common substring and the matches in the
// untouched runes to its left and right.
func matched(a, b []rune) int {
	i, j, size := longestMatch(a, b)
	if size == 0 {
		return 0
	}
	return size + matched(a[:i], b[:j]) + matched(a[i+size:], b[j+size:])
}

// longestMatch returns the start offsets and length of the longest common
// substring, preferring the earliest occurrence in a and then in b — the same
// tie-break difflib makes.
func longestMatch(a, b []rune) (int, int, int) {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	bestI, bestJ, bestSize := 0, 0, 0
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] != b[j-1] {
				cur[j] = 0
				continue
			}
			cur[j] = prev[j-1] + 1
			if cur[j] > bestSize {
				bestSize = cur[j]
				bestI, bestJ = i-cur[j], j-cur[j]
			}
		}
		prev, cur = cur, prev
		clear(cur)
	}
	return bestI, bestJ, bestSize
}
