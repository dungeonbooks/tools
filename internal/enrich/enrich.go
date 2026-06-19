// Package enrich resolves one book to rich metadata, layering Hardcover
// (ratings, genres), Google Books (broad coverage), and OpenLibrary (fallback).
package enrich

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/dungeonbooks/tools/internal/bookmeta"
)

// Source names for forcing a single provider (default "" = merged waterfall).
const (
	SourceHardcover   = "hardcover"
	SourceGoogle      = "google"
	SourceOpenLibrary = "openlibrary"
)

// Source is a keyless-style metadata source (Google Books, OpenLibrary).
type Source interface {
	ByISBN(ctx context.Context, isbn string) (bookmeta.Book, error)
	Search(ctx context.Context, query string) (bookmeta.Book, error)
}

// HCSource is Hardcover, which adds reader-signal data and gates on a token.
type HCSource interface {
	Enabled() bool
	ByISBN(ctx context.Context, isbn string) (bookmeta.Book, error)
	SearchTop(ctx context.Context, query string) (bookmeta.Book, error)
}

type Service struct {
	hc HCSource
	gb Source
	ol Source
}

func NewService(hc HCSource, gb, ol Source) *Service {
	return &Service{hc: hc, gb: gb, ol: ol}
}

// New wires the real Hardcover, Google Books, and OpenLibrary clients.
func New(hardcoverToken, googleKey string) *Service {
	hc := &http.Client{Timeout: 15 * time.Second}
	return NewService(NewHardcover(hardcoverToken, hc), NewGoogleBooks(googleKey, hc), NewOpenLibrary(hc))
}

// needsMore reports whether a book is missing fields worth filling from another source.
func needsMore(b bookmeta.Book) bool {
	return b.Title == "" || b.CoverURL == "" || b.Description == ""
}

// Book resolves a query (ISBN or phrase) to one enriched book. source forces a
// single provider for comparison; "" runs the merged waterfall.
func (s *Service) Book(ctx context.Context, query, source string) (bookmeta.Book, error) {
	isbn := ""
	if n := bookmeta.NormalizeISBN(query); bookmeta.PlausibleISBN13(n) {
		isbn = n
	}
	switch source {
	case "":
		return s.auto(ctx, query, isbn)
	case SourceHardcover:
		if !s.hc.Enabled() {
			return bookmeta.Book{}, errors.New("hardcover source unavailable: set HARDCOVER_API_TOKEN")
		}
		if isbn != "" {
			return s.hc.ByISBN(ctx, isbn)
		}
		return s.hc.SearchTop(ctx, query)
	case SourceGoogle:
		return single(ctx, s.gb, query, isbn)
	case SourceOpenLibrary:
		return single(ctx, s.ol, query, isbn)
	default:
		return bookmeta.Book{}, fmt.Errorf("unknown source %q (use hardcover, google, or openlibrary)", source)
	}
}

func single(ctx context.Context, src Source, query, isbn string) (bookmeta.Book, error) {
	if isbn != "" {
		return src.ByISBN(ctx, isbn)
	}
	return src.Search(ctx, query)
}

// auto runs Hardcover -> Google -> OpenLibrary, stopping once cover + description fill.
func (s *Service) auto(ctx context.Context, query, isbn string) (bookmeta.Book, error) {
	if isbn != "" {
		var b bookmeta.Book
		if s.hc.Enabled() {
			if hcb, err := s.hc.ByISBN(ctx, isbn); err == nil {
				b = hcb
			}
		}
		if needsMore(b) {
			if gbb, err := s.gb.ByISBN(ctx, isbn); err == nil {
				b.Fill(gbb)
			}
		}
		if needsMore(b) {
			if olb, err := s.ol.ByISBN(ctx, isbn); err == nil {
				b.Fill(olb)
			}
		}
		if b.ISBN13 == "" {
			b.ISBN13 = isbn
		}
		return b, nil
	}

	var b bookmeta.Book
	if s.hc.Enabled() {
		if hcb, err := s.hc.SearchTop(ctx, query); err == nil {
			b = hcb
		}
	}
	if b.Title == "" {
		if gbb, err := s.gb.Search(ctx, query); err == nil {
			b = gbb
		}
	}
	if b.Title == "" {
		return s.ol.Search(ctx, query)
	}
	if needsMore(b) && b.ISBN13 != "" {
		if gbb, err := s.gb.ByISBN(ctx, b.ISBN13); err == nil {
			b.Fill(gbb)
		}
	}
	return b, nil
}
