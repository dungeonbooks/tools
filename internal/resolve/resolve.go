// Package resolve turns a book title into a *verified* ISBN-13.
//
// A bare lookup is not trustworthy on its own. Two failure modes show up
// constantly against real metadata providers:
//
//   - A bare title mismatches badly. "Playing at the World" alone resolves to
//     King Lear; "Unboxed" to a Dr. Seuss box set. The author disambiguates,
//     so callers should always supply one.
//   - The default source merge occasionally returns a confidently wrong book
//     whose title barely matches the request.
//
// So every result is scored against what was asked for, weak matches are
// retried against single sources, and anything below the confidence floor is
// returned marked unverified rather than passed off as an answer. Some books
// are in no source at all; unverified is the correct outcome for those, not a
// bug to work around.
package resolve

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/dungeonbooks/tools/internal/bookmeta"
	"github.com/dungeonbooks/tools/internal/enrich"
)

// ConfidenceFloor is the score at or above which a match is called verified.
const ConfidenceFloor = 0.60

// strongMatch short-circuits the source fallback: a result this good will not
// be improved on, so we stop spending round trips.
const strongMatch = 0.85

// authorMismatchPenalty is subtracted when the requested author does not agree
// with the matched book's. It is a penalty rather than a rejection so a caller
// can still see the best guess alongside verified: false.
const authorMismatchPenalty = 0.25

// sources are tried in order: the default merge first, then single sources as
// fallbacks for when the merge picks the wrong book.
var sources = []string{"", enrich.SourceGoogle, enrich.SourceOpenLibrary}

// Lookup is the slice of enrich.Service this package needs.
type Lookup interface {
	Book(ctx context.Context, query, source string) (bookmeta.Book, error)
}

// Result is the contract. Verified is the only field a caller should branch on:
// when it is false the ISBN13 (if any) is a best guess that failed
// verification, and must not be treated as this book's ISBN.
type Result struct {
	Query      string  `json:"query" jsonschema:"the query that was issued to the metadata providers"`
	Verified   bool    `json:"verified" jsonschema:"true only if the returned book was confirmed to be the book that was asked for; if false, do not use isbn13"`
	Confidence float64 `json:"confidence" jsonschema:"match score from 0 to 1; below 0.6 is unverified"`
	Reason     string  `json:"reason,omitempty" jsonschema:"why the result is unverified; empty when verified"`
	// Retryable separates "a provider failed" from "this book does not exist" —
	// two facts that look identical from the outside and mean opposite things.
	Retryable    bool    `json:"retryable,omitempty" jsonschema:"true when a provider errored or timed out, so the lookup failed rather than the book being absent; retry before concluding anything about the book"`
	ISBN13       string  `json:"isbn13,omitempty" jsonschema:"the ISBN-13, trustworthy only when verified is true"`
	Title        string  `json:"title,omitempty" jsonschema:"title as the provider records it, which may carry a subtitle"`
	Author       string  `json:"author,omitempty"`
	Year         int     `json:"year,omitempty"`
	Rating       float64 `json:"rating,omitempty" jsonschema:"Hardcover community rating; present only when a Hardcover token is configured"`
	RatingsCount int     `json:"ratings_count,omitempty"`
	HardcoverURL string  `json:"hardcover_url,omitempty"`
}

// Describe renders the book for a human, omitting whatever the providers did not
// supply. Not every record carries an author or a year, and "Some Title — (0)"
// reads as a bug in the tool rather than a gap in the catalogue.
func (r Result) Describe() string {
	s := r.Title
	if s == "" {
		s = "(untitled)"
	}
	if r.Author != "" {
		s += " — " + r.Author
	}
	if r.Year > 0 {
		s += fmt.Sprintf(" (%d)", r.Year)
	}
	return s
}

func fromBook(b bookmeta.Book) Result {
	return Result{
		ISBN13:       b.ISBN13,
		Title:        b.Title,
		Author:       b.Author,
		Year:         b.Year,
		Rating:       b.Rating,
		RatingsCount: b.RatingsCount,
		HardcoverURL: b.HardcoverURL,
	}
}

// Title resolves a title, optionally disambiguated by an author, to a verified
// ISBN-13. Pass the author whenever it is known: without it there is nothing to
// gate a plausible-looking wrong match on.
func Title(ctx context.Context, lk Lookup, title, author string) Result {
	query := title
	if author != "" {
		query += " " + author
	}

	var best bookmeta.Book
	var failures []error
	bestScore := math.Inf(-1)
	found, malformed := false, false
	for _, src := range sources {
		b, err := lk.Book(ctx, query, src)
		if err != nil {
			// A provider that finds nothing returns an empty book and no error,
			// so an error here is a failed request, not an absent book.
			failures = append(failures, err)
			continue
		}
		// The providers only gate on a *plausible* ISBN (length and prefix), so a
		// corrupt one reaches us. Verifying a book against an ISBN that cannot
		// exist would hand back a number that fails at the first scanner.
		if !bookmeta.ValidISBN13(bookmeta.NormalizeISBN(b.ISBN13)) {
			malformed = malformed || b.ISBN13 != ""
			continue
		}
		score := titleScore(title, b.Title)
		if author != "" && !bookmeta.AuthorsMatch(author, b.Author) {
			score -= authorMismatchPenalty
		}
		if score > bestScore {
			best, bestScore, found = b, score, true
		}
		if score >= strongMatch {
			break
		}
	}
	if !found {
		return notFound(query, failures, malformed)
	}

	r := fromBook(best)
	r.Query = query
	r.Confidence = trunc2(math.Max(bestScore, 0))
	r.Verified = r.Confidence >= ConfidenceFloor
	if !r.Verified {
		r.Reason = fmt.Sprintf("weak title/author match (confidence %.2f < %.2f)", r.Confidence, ConfidenceFloor)
	}
	return r
}

// ISBN verifies that an ISBN-13 names a real book and returns its metadata. A
// provider that cannot find the ISBN may fall back to a search and hand back a
// different book; that is caught here and reported as unverified.
func ISBN(ctx context.Context, lk Lookup, isbn string) Result {
	want := bookmeta.NormalizeISBN(isbn)
	if !bookmeta.ValidISBN13(want) {
		return Result{
			Query:  isbn,
			Reason: "not a valid ISBN-13 (need 13 digits with a correct check digit)",
		}
	}

	b, err := lk.Book(ctx, want, "")
	if err != nil {
		return notFound(isbn, []error{err}, false)
	}
	got := bookmeta.NormalizeISBN(b.ISBN13)
	if got == "" {
		return Result{Query: isbn, Reason: "no book found for that ISBN"}
	}

	r := fromBook(b)
	r.Query = isbn
	if got != want {
		r.Reason = fmt.Sprintf("provider returned a different ISBN (%s)", b.ISBN13)
		return r
	}
	r.Verified = true
	r.Confidence = 1
	return r
}

// notFound reports a lookup that produced nothing, and says which kind of
// nothing. A rate-limited provider and a book that no catalogue carries both
// come back empty, but "retry" and "this book has no ISBN" are opposite
// instructions, and a caller that cannot tell them apart will confidently
// report an outage as a fact about the book.
func notFound(query string, failures []error, malformed bool) Result {
	if len(failures) > 0 {
		return Result{
			Query:     query,
			Retryable: true,
			Reason: fmt.Sprintf("the lookup failed: %d of %d sources errored (%v). This says nothing about whether the book exists — retry before concluding it is unfindable",
				len(failures), len(sources), errors.Join(failures...)),
		}
	}
	if malformed {
		return Result{
			Query:  query,
			Reason: "a source returned an ISBN-13 that fails its check digit, so it cannot be a real ISBN and was rejected",
		}
	}
	return Result{Query: query, Reason: "no source returned a result with an ISBN"}
}

// trunc2 cuts the score to the two decimals that get shown, rather than rounding
// to nearest, and Verified is then decided on the result — so the number a caller
// reads and the verdict it gets are the same number. Rounding to nearest would
// let 0.599 print as "confidence 0.60 < 0.60", and deciding on the rounded value
// would nudge that same match up over the floor. Truncating can only lower a
// score, and loosening this gate is the direction that hands back a wrong ISBN.
func trunc2(f float64) float64 {
	return math.Trunc(f*100) / 100
}
