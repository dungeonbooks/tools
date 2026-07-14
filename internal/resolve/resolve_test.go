package resolve

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/dungeonbooks/tools/internal/bookmeta"
	"github.com/dungeonbooks/tools/internal/enrich"
)

// stub answers per source and records the order it was asked. It mirrors the
// contract enrich actually has: a source that finds nothing returns an empty
// book and no error, so an error means the request failed. Conflating the two is
// the bug TestTitleSeparatesAFailedLookupFromAnAbsentBook guards.
type stub struct {
	bySource map[string]bookmeta.Book
	errs     map[string]error
	asked    []string
}

func (s *stub) Book(_ context.Context, _, source string) (bookmeta.Book, error) {
	s.asked = append(s.asked, source)
	if err, ok := s.errs[source]; ok {
		return bookmeta.Book{}, err
	}
	return s.bySource[source], nil
}

func TestTitleVerifiesAStrongMatchAndStopsEarly(t *testing.T) {
	lk := &stub{bySource: map[string]bookmeta.Book{
		"": {ISBN13: "9780262376303", Title: "The Beauty of Games", Author: "Frank Lantz", Year: 2023},
	}}

	r := Title(context.Background(), lk, "The Beauty of Games", "Frank Lantz")

	if !r.Verified {
		t.Fatalf("want verified, got unverified: %s", r.Reason)
	}
	if r.ISBN13 != "9780262376303" {
		t.Errorf("ISBN13 = %q, want 9780262376303", r.ISBN13)
	}
	if r.Query != "The Beauty of Games Frank Lantz" {
		t.Errorf("Query = %q, want the author folded in", r.Query)
	}
	if len(lk.asked) != 1 {
		t.Errorf("asked %v sources, want 1: a strong match must not spend fallback calls", lk.asked)
	}
}

// The subtitle case the character-level backstop exists for: token precision is
// poor against a long subtitle, so containment alone would sink this.
func TestTitleAcceptsASubtitledEdition(t *testing.T) {
	lk := &stub{bySource: map[string]bookmeta.Book{
		"": {ISBN13: "9780262542951", Title: "Playing at the World: A History of Simulating Wars", Author: "Jon Peterson"},
	}}

	r := Title(context.Background(), lk, "Playing at the World", "Jon Peterson")

	if !r.Verified {
		t.Fatalf("want verified for a subtitled edition, got %s (confidence %.2f)", r.Reason, r.Confidence)
	}
}

// The documented failure: a bare title confidently returning the wrong book.
func TestTitleRejectsAConfidentWrongMatch(t *testing.T) {
	kingLear := bookmeta.Book{ISBN13: "9780743482769", Title: "King Lear", Author: "William Shakespeare"}
	lk := &stub{bySource: map[string]bookmeta.Book{
		"":                       kingLear,
		enrich.SourceGoogle:      kingLear,
		enrich.SourceOpenLibrary: kingLear,
	}}

	r := Title(context.Background(), lk, "Playing at the World", "Jon Peterson")

	if r.Verified {
		t.Fatalf("King Lear passed verification for %q", r.Query)
	}
	if r.ISBN13 != kingLear.ISBN13 {
		t.Errorf("ISBN13 = %q, want the best guess retained for the caller to inspect", r.ISBN13)
	}
	if r.Reason == "" {
		t.Error("Reason is empty; an unverified result must say why")
	}
	if len(lk.asked) != len(sources) {
		t.Errorf("asked %v, want every source tried before giving up", lk.asked)
	}
}

// The dangerous direction. A sequel shares its predecessor's title as a prefix,
// carries the same author, and so clears every gate except the one that notices
// the extra word. Verifying one of these hands back a real ISBN for the wrong
// book — the single failure this package exists to prevent.
func TestTitleRejectsASequelByTheSameAuthor(t *testing.T) {
	sequels := map[string]struct{ title, author, got string }{
		"Foundation": {"Foundation", "Isaac Asimov", "Foundation and Empire"},
		"Dune":       {"Dune", "Frank Herbert", "Dune Messiah"},
	}
	for name, tc := range sequels {
		t.Run(name, func(t *testing.T) {
			lk := &stub{bySource: map[string]bookmeta.Book{
				"":                       {ISBN13: "9780553293371", Title: tc.got, Author: tc.author},
				enrich.SourceGoogle:      {ISBN13: "9780553293371", Title: tc.got, Author: tc.author},
				enrich.SourceOpenLibrary: {ISBN13: "9780553293371", Title: tc.got, Author: tc.author},
			}}

			r := Title(context.Background(), lk, tc.title, tc.author)

			if r.Verified {
				t.Fatalf("%q verified against %q (confidence %.2f): that is the wrong book's ISBN",
					tc.title, tc.got, r.Confidence)
			}
		})
	}
}

// A short title inside a longer unrelated one must not win on containment.
func TestTitleRejectsContainmentOfAShortTitle(t *testing.T) {
	lk := &stub{bySource: map[string]bookmeta.Book{
		"": {ISBN13: "9781501110368", Title: "It Ends With Us", Author: "Colleen Hoover"},
	}}

	r := Title(context.Background(), lk, "It", "Stephen King")

	if r.Verified {
		t.Fatalf("%q matched %q with confidence %.2f", "It", "It Ends With Us", r.Confidence)
	}
}

func TestTitleFallsBackToASingleSourceWhenTheMergeIsWeak(t *testing.T) {
	// The merge result carries a perfectly valid ISBN — it is out-scored on the
	// title, not skipped for being malformed, which is what this test is for.
	lk := &stub{bySource: map[string]bookmeta.Book{
		"":                  {ISBN13: "9780743482769", Title: "A Romance in Provence", Author: "Someone Else"},
		enrich.SourceGoogle: {ISBN13: "9780262542951", Title: "Playing at the World", Author: "Jon Peterson"},
	}}

	r := Title(context.Background(), lk, "Playing at the World", "Jon Peterson")

	if !r.Verified {
		t.Fatalf("want the google fallback to win, got %s", r.Reason)
	}
	if r.ISBN13 != "9780262542951" {
		t.Errorf("ISBN13 = %q, want the better-scoring fallback to be kept", r.ISBN13)
	}
}

func TestTitleWithNoISBNAnywhereIsUnverified(t *testing.T) {
	lk := &stub{bySource: map[string]bookmeta.Book{
		"": {Title: "Playing Place", Author: "Chad Randl"}, // in the index, but carries no ISBN
	}}

	r := Title(context.Background(), lk, "Playing Place", "Chad Randl")

	if r.Verified {
		t.Fatal("a book with no ISBN cannot be a verified ISBN lookup")
	}
	if r.ISBN13 != "" {
		t.Errorf("ISBN13 = %q, want empty", r.ISBN13)
	}
	if r.Reason == "" {
		t.Error("Reason is empty")
	}
}

// A rate-limited provider and a book no catalogue carries both come back empty.
// Reporting them the same way is how an outage gets recorded as a fact about a
// book — which is exactly what happened to "Playing Place" by Chad Randl, a book
// that does exist (9780262047838) and was written off as absent.
func TestTitleSeparatesAFailedLookupFromAnAbsentBook(t *testing.T) {
	absent := &stub{bySource: map[string]bookmeta.Book{}} // every source answers, none has it
	broken := &stub{errs: map[string]error{
		"":                       errors.New("googlebooks: status 429"),
		enrich.SourceGoogle:      errors.New("googlebooks: status 429"),
		enrich.SourceOpenLibrary: errors.New("openlibrary: status 503"),
	}}

	gone := Title(context.Background(), absent, "Playing Place", "Chad Randl")
	failed := Title(context.Background(), broken, "Playing Place", "Chad Randl")

	if gone.Verified || failed.Verified {
		t.Fatal("neither case can be verified")
	}
	if gone.Retryable {
		t.Error("an absent book is not retryable: retrying will never find it")
	}
	if !failed.Retryable {
		t.Error("a provider outage must be retryable, not reported as an absent book")
	}
	if !strings.Contains(failed.Reason, "429") {
		t.Errorf("Reason does not surface the provider error: %q", failed.Reason)
	}
	if gone.Reason == failed.Reason {
		t.Errorf("an outage and an absent book give the same reason %q; a caller cannot tell them apart", gone.Reason)
	}
}

func TestISBNReportsAFailedLookupAsRetryable(t *testing.T) {
	broken := &stub{errs: map[string]error{"": errors.New("hardcover: status 502")}}

	r := ISBN(context.Background(), broken, "9780262542951")

	if r.Verified {
		t.Fatal("a failed lookup cannot verify")
	}
	if !r.Retryable {
		t.Error("a provider outage must be retryable, not reported as an unknown ISBN")
	}
}

// The confidence a caller reads and the verdict it gets must be the same number.
// Rounding to nearest let a 0.599 match report "confidence 0.60 < 0.60".
func TestConfidenceAgreesWithTheVerdictAtTheFloor(t *testing.T) {
	for _, score := range []float64{0.599, 0.5999999, 0.6, 0.601, 0.0, 1.0} {
		got := trunc2(score)
		if verified := got >= ConfidenceFloor; verified != (score >= ConfidenceFloor) && got != score {
			// A truncated score may fall below the floor where the raw one did
			// not only if truncation lowered it — never the other way.
			if got > score {
				t.Errorf("trunc2(%v) = %v rounded the score up, which can loosen the gate", score, got)
			}
		}
		if got > score {
			t.Errorf("trunc2(%v) = %v, want a value no greater than the input", score, got)
		}
	}

	// The reported figure never contradicts the verdict it is printed beside.
	lk := &stub{bySource: map[string]bookmeta.Book{
		"": {ISBN13: "9781501110368", Title: "It Ends With Us", Author: "Colleen Hoover"},
	}}
	r := Title(context.Background(), lk, "It", "Stephen King")
	if r.Verified != (r.Confidence >= ConfidenceFloor) {
		t.Errorf("Verified = %v but Confidence %.2f says otherwise", r.Verified, r.Confidence)
	}
}

// The providers gate on a *plausible* ISBN (length and prefix) but never check
// the check digit, so a corrupt number reaches us. Verifying against one would
// hand back an ISBN that fails at the first scanner.
func TestTitleRejectsAMalformedISBNFromAProvider(t *testing.T) {
	lk := &stub{bySource: map[string]bookmeta.Book{
		// Right book, right author, but the check digit is wrong.
		"":                       {ISBN13: "9780262376300", Title: "The Beauty of Games", Author: "Frank Lantz"},
		enrich.SourceGoogle:      {ISBN13: "9780262376300", Title: "The Beauty of Games", Author: "Frank Lantz"},
		enrich.SourceOpenLibrary: {ISBN13: "9780262376300", Title: "The Beauty of Games", Author: "Frank Lantz"},
	}}

	r := Title(context.Background(), lk, "The Beauty of Games", "Frank Lantz")

	if r.Verified {
		t.Fatalf("verified a malformed ISBN %q on an otherwise perfect match", r.ISBN13)
	}
	if !strings.Contains(r.Reason, "check digit") {
		t.Errorf("Reason = %q, want it to name the check digit", r.Reason)
	}
	if r.Retryable {
		t.Error("a malformed ISBN is not a transient failure; retrying returns the same bad number")
	}
}

// Not every record carries an author or a year. "Some Title — (0)" reads as a bug
// in the tool rather than a gap in the catalogue.
func TestDescribeOmitsFieldsTheProvidersDidNotSupply(t *testing.T) {
	tests := []struct {
		r    Result
		want string
	}{
		{Result{Title: "Dune", Author: "Frank Herbert", Year: 1965}, "Dune — Frank Herbert (1965)"},
		{Result{Title: "Dune", Author: "Frank Herbert"}, "Dune — Frank Herbert"},
		{Result{Title: "Dune", Year: 1965}, "Dune (1965)"},
		{Result{Title: "Dune"}, "Dune"},
		{Result{}, "(untitled)"},
	}
	for _, tt := range tests {
		if got := tt.r.Describe(); got != tt.want {
			t.Errorf("Describe() = %q, want %q", got, tt.want)
		}
	}
}

func TestTitlePenalizesAnAuthorMismatch(t *testing.T) {
	book := bookmeta.Book{ISBN13: "9780262542951", Title: "Playing at the World", Author: "Jon Peterson"}
	lk := func() *stub {
		return &stub{bySource: map[string]bookmeta.Book{
			"": book, enrich.SourceGoogle: book, enrich.SourceOpenLibrary: book,
		}}
	}

	right := Title(context.Background(), lk(), "Playing at the World", "Jon Peterson")
	wrong := Title(context.Background(), lk(), "Playing at the World", "Ursula K. Le Guin")

	if got := right.Confidence - wrong.Confidence; math.Abs(got-authorMismatchPenalty) > 0.01 {
		t.Errorf("author mismatch cost %.2f confidence, want %.2f", got, authorMismatchPenalty)
	}
}

func TestISBNVerifiesARoundTrip(t *testing.T) {
	lk := &stub{bySource: map[string]bookmeta.Book{
		"": {ISBN13: "9780262542951", Title: "Playing at the World", Author: "Jon Peterson"},
	}}

	r := ISBN(context.Background(), lk, "978-0-262-54295-1")

	if !r.Verified {
		t.Fatalf("want verified, got %s", r.Reason)
	}
	if r.Confidence != 1 {
		t.Errorf("Confidence = %v, want 1", r.Confidence)
	}
}

func TestISBNRejectsABadCheckDigitWithoutACall(t *testing.T) {
	lk := &stub{}

	r := ISBN(context.Background(), lk, "9780262542950")

	if r.Verified {
		t.Fatal("an ISBN with a bad check digit was verified")
	}
	if len(lk.asked) != 0 {
		t.Errorf("made %d provider calls, want 0: a malformed ISBN is rejected locally", len(lk.asked))
	}
}

// A provider that cannot find the ISBN may fall back to a search and hand back
// some other book. The returned ISBN must be checked against the one requested.
func TestISBNRejectsADifferentBook(t *testing.T) {
	lk := &stub{bySource: map[string]bookmeta.Book{
		"": {ISBN13: "9780743482769", Title: "King Lear", Author: "William Shakespeare"},
	}}

	r := ISBN(context.Background(), lk, "9780262542951")

	if r.Verified {
		t.Fatal("a different book passed ISBN verification")
	}
	if r.Reason == "" {
		t.Error("Reason is empty")
	}
}

func TestValidISBN13(t *testing.T) {
	tests := map[string]bool{
		"9780262542951": true,
		"9780262542950": false, // check digit off by one
		"9790262542950": true,  // 979 prefix is legal
		"9790262542951": false, // 979 prefix, wrong check digit
		"1234567890123": false, // no 978/979 prefix
		"978026254295":  false, // twelve digits
		"978026254295X": false, // ISBN-10 check char in an ISBN-13
	}
	for isbn, want := range tests {
		if got := bookmeta.ValidISBN13(isbn); got != want {
			t.Errorf("ValidISBN13(%q) = %v, want %v", isbn, got, want)
		}
	}
}

// ratio is a port of difflib.SequenceMatcher.ratio; these expectations come
// from CPython's difflib, so a regression in the port shows up here.
func TestRatioMatchesDifflib(t *testing.T) {
	tests := []struct {
		a, b string
		want float64
	}{
		{"", "", 1},
		{"abc", "abc", 1},
		{"abc", "xyz", 0},
		{"playing at the world", "playing at the world a history of simulating wars", 0.5797101449275363},
		{"it", "it ends with us", 0.23529411764705882},
		{"the beauty of games", "the beauty of games", 1},
	}
	for _, tt := range tests {
		if got := ratio(tt.a, tt.b); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("ratio(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestTitleScoreSeparatesSubtitlesFromDifferentBooks(t *testing.T) {
	tests := []struct {
		want, got string
		pass      bool
		note      string
	}{
		{"The Beauty of Games", "The Beauty of Games", true, "exact"},
		{"Playing at the World", "Playing at the World: A History of Simulating Wars", true, "subtitle"},
		{"Playing at the World", "Playing at the World, 2E, Volume 1", true, "edition and volume markers (how Hardcover files it)"},
		{"Dune", "Dune, Revised Edition", true, "edition marker on a one-word title"},
		{"Sapiens", "Sapiens: A Brief History of Humankind", true, "subtitle on a one-word title"},
		{"The Hobbit", "The Hobbit, or There and Back Again", true, "alternate title"},
		{"The Colour of Magic", "Colour of Magic", true, "dropped article"},
		{"The Beauty of Gaems", "The Beauty of Games", true, "typo in the request"},
		{"Playing at the World", "King Lear", false, "unrelated book"},
		{"Unboxed", "The Big Green Box of Beginner Books", false, "unrelated book"},
		{"It", "It Ends With Us", false, "title contained in a longer one"},
		{"Emma", "Emma in the Night", false, "title contained in a longer one"},
		{"Dune", "Dune Messiah", false, "sequel"},
		{"Foundation", "Foundation and Empire", false, "sequel"},
	}
	for _, tt := range tests {
		got := titleScore(tt.want, tt.got)
		if pass := got >= ConfidenceFloor; pass != tt.pass {
			t.Errorf("titleScore(%q, %q) = %.3f (%s), want %s the %.2f floor — %s",
				tt.want, tt.got, got,
				map[bool]string{true: "pass", false: "fail"}[pass],
				map[bool]string{true: "at or above", false: "below"}[tt.pass],
				ConfidenceFloor, tt.note)
		}
	}
}

func TestNormalizeCollapsesPunctuationAndCase(t *testing.T) {
	tests := map[string]string{
		"The Beauty of Games":  "the beauty of games",
		"  Hello,   World!  ":  "hello world",
		"J.R.R. Tolkien":       "j r r tolkien",
		"":                     "",
		"!!!":                  "",
		"Don't Panic — Really": "don t panic really",
	}
	for in, want := range tests {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
