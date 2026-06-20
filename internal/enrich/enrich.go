package enrich

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/dungeonbooks/tools/internal/bookmeta"
)

const (
	SourceHardcover   = "hardcover"
	SourceGoogle      = "google"
	SourceOpenLibrary = "openlibrary"
)

type Source interface {
	ByISBN(ctx context.Context, isbn string) (bookmeta.Book, error)
	Search(ctx context.Context, query string) (bookmeta.Book, error)
}

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

func New(hardcoverToken, googleKey string) *Service {
	hc := &http.Client{Timeout: 15 * time.Second}
	return NewService(NewHardcover(hardcoverToken, hc), NewGoogleBooks(googleKey, hc), NewOpenLibrary(hc))
}

func needsMore(b bookmeta.Book) bool {
	return b.Title == "" || b.CoverURL == "" || b.Description == ""
}

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

// ISBN resolves a title (optionally with author) to an ISBN-13, returning ""
// when no confident match carries one. It reuses the auto lookup, so it benefits
// from the same Hardcover/Google/OpenLibrary fallback chain as Book.
//
// When an author is supplied it acts as a confidence gate: the matched book's
// author must agree, otherwise we discard the ISBN rather than risk pinning an
// obscure or pre-publication title to the wrong edition. Without an author we
// can't gate, so the top match is accepted as-is.
func (s *Service) ISBN(ctx context.Context, title, author string) (string, error) {
	query := title
	if author != "" {
		query += " " + author
	}
	b, err := s.Book(ctx, query, "")
	if err != nil {
		return "", err
	}
	if author != "" && !bookmeta.AuthorsMatch(author, b.Author) {
		return "", nil
	}
	return b.ISBN13, nil
}

func single(ctx context.Context, src Source, query, isbn string) (bookmeta.Book, error) {
	if isbn != "" {
		return src.ByISBN(ctx, isbn)
	}
	return src.Search(ctx, query)
}

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
