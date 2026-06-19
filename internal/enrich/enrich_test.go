package enrich

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dungeonbooks/tools/internal/bookmeta"
)

type fakeOL struct{ byISBN, search bookmeta.Book }

func (f fakeOL) ByISBN(context.Context, string) (bookmeta.Book, error) { return f.byISBN, nil }
func (f fakeOL) Search(context.Context, string) (bookmeta.Book, error) { return f.search, nil }

type fakeHC struct {
	enabled     bool
	top, byISBN bookmeta.Book
}

func (f fakeHC) Enabled() bool                                            { return f.enabled }
func (f fakeHC) SearchTop(context.Context, string) (bookmeta.Book, error) { return f.top, nil }
func (f fakeHC) ByISBN(context.Context, string) (bookmeta.Book, error)    { return f.byISBN, nil }

func TestBookISBNUsesHardcoverThenFillsFromOL(t *testing.T) {
	// OL lacks the new book's description/rating; Hardcover has them. OL still
	// supplies the cover and page count.
	ol := fakeOL{byISBN: bookmeta.Book{CoverURL: "c", PageCount: 300}}
	hc := fakeHC{enabled: true, byISBN: bookmeta.Book{Title: "X", Author: "A", ISBN13: "9780593128282", Rating: 4.2, RatingsCount: 10, Description: "d", Subjects: []string{"Fantasy"}}}
	b, err := NewService(hc, fakeOL{}, ol).Book(context.Background(), "9780593128282", "")
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "X" || b.Rating != 4.2 || b.Description != "d" || b.CoverURL != "c" || b.PageCount != 300 {
		t.Fatalf("bad merge: %+v", b)
	}
}

func TestBookPhraseFallsBackToOLWhenHardcoverDisabled(t *testing.T) {
	ol := fakeOL{search: bookmeta.Book{Title: "S"}}
	b, err := NewService(fakeHC{enabled: false}, fakeOL{}, ol).Book(context.Background(), "some phrase", "")
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "S" {
		t.Fatalf("expected OL search result, got %+v", b)
	}
}

func TestBookSourceForcesSingleProvider(t *testing.T) {
	svc := NewService(
		fakeHC{enabled: true, byISBN: bookmeta.Book{Title: "FROM_HC"}},
		fakeOL{byISBN: bookmeta.Book{Title: "FROM_GB"}},
		fakeOL{byISBN: bookmeta.Book{Title: "FROM_OL"}},
	)
	for src, want := range map[string]string{"hardcover": "FROM_HC", "google": "FROM_GB", "openlibrary": "FROM_OL"} {
		b, err := svc.Book(context.Background(), "9780593128282", src)
		if err != nil {
			t.Fatal(err)
		}
		if b.Title != want {
			t.Fatalf("source %q: got %q, want %q", src, b.Title, want)
		}
	}
	if _, err := svc.Book(context.Background(), "x", "bogus"); err == nil {
		t.Fatal("expected error for unknown source")
	}
}

func TestGoogleBooksByISBNParses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"items":[{"volumeInfo":{"title":"The Unicorn Hunters","authors":["Katherine Arden"],"description":"d","pageCount":352,"publishedDate":"2026-06-02","categories":["Fiction"],"averageRating":4.0,"ratingsCount":5,"imageLinks":{"thumbnail":"http://cover"},"industryIdentifiers":[{"type":"ISBN_13","identifier":"9780593128282"}]}}]}`))
	}))
	defer srv.Close()
	g := NewGoogleBooks("", srv.Client())
	g.base = srv.URL
	b, err := g.ByISBN(context.Background(), "9780593128282")
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "The Unicorn Hunters" || b.Author != "Katherine Arden" || b.Year != 2026 || b.PageCount != 352 || b.ISBN13 != "9780593128282" || b.CoverURL != "http://cover" {
		t.Fatalf("bad parse: %+v", b)
	}
}

func TestOpenLibraryByISBNParses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"ISBN:9780593128282":{"title":"The Unicorn Hunters","authors":[{"name":"Katherine Arden"}],"cover":{"large":"https://cover"},"subjects":[{"name":"Fantasy"}],"number_of_pages":352,"publish_date":"June 2, 2026"}}`))
	}))
	defer srv.Close()
	o := NewOpenLibrary(srv.Client())
	o.base = srv.URL
	b, err := o.ByISBN(context.Background(), "9780593128282")
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "The Unicorn Hunters" || b.Author != "Katherine Arden" || b.PageCount != 352 || b.Year != 2026 || b.CoverURL != "https://cover" {
		t.Fatalf("bad parse: %+v", b)
	}
}

func TestHardcoverSearchTopParses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		if strings.Contains(string(buf), "SearchBooks") {
			w.Write([]byte(`{"data":{"search":{"ids":[42]}}}`))
			return
		}
		w.Write([]byte(`{"data":{"books_by_pk":{"title":"Sublimation","rating":4.5,"ratings_count":12,"pages":320,"release_year":2026,"slug":"sublimation","cached_tags":{"Genre":[{"tag":"Science Fiction"}]},"contributions":[{"author":{"name":"Isabel J. Kim"}}],"editions":[{"isbn_13":"9781250799609"}]}}}`))
	}))
	defer srv.Close()
	h := NewHardcover("tok", srv.Client())
	h.url = srv.URL
	b, err := h.SearchTop(context.Background(), "sublimation")
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "Sublimation" || b.Rating != 4.5 || b.Author != "Isabel J. Kim" || b.ISBN13 != "9781250799609" {
		t.Fatalf("bad parse: %+v", b)
	}
	if len(b.Subjects) != 1 || b.Subjects[0] != "Science Fiction" {
		t.Fatalf("bad genres: %+v", b.Subjects)
	}
	if b.HardcoverURL != "https://hardcover.app/books/sublimation" {
		t.Fatalf("bad url: %s", b.HardcoverURL)
	}
}
