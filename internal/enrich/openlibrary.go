package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/dungeonbooks/tools/internal/bookmeta"
)

type OpenLibrary struct {
	http *http.Client
	base string // override for tests; defaults to https://openlibrary.org
}

func NewOpenLibrary(hc *http.Client) *OpenLibrary {
	return &OpenLibrary{http: hc, base: "https://openlibrary.org"}
}

var yearRE = regexp.MustCompile(`\b(\d{4})\b`)

func (o *OpenLibrary) ByISBN(ctx context.Context, isbn string) (bookmeta.Book, error) {
	u := fmt.Sprintf("%s/api/books?bibkeys=ISBN:%s&format=json&jscmd=data", o.base, url.QueryEscape(isbn))
	var raw map[string]struct {
		Title   string `json:"title"`
		Authors []struct {
			Name string `json:"name"`
		} `json:"authors"`
		Cover struct {
			Large string `json:"large"`
		} `json:"cover"`
		Subjects []struct {
			Name string `json:"name"`
		} `json:"subjects"`
		Pages      int `json:"number_of_pages"`
		Publishers []struct {
			Name string `json:"name"`
		} `json:"publishers"`
		PublishDate string `json:"publish_date"`
	}
	if err := o.getJSON(ctx, u, &raw); err != nil {
		return bookmeta.Book{}, err
	}
	d, ok := raw["ISBN:"+isbn]
	if !ok {
		return bookmeta.Book{}, nil
	}
	b := bookmeta.Book{ISBN13: isbn, Title: d.Title, CoverURL: d.Cover.Large, PageCount: d.Pages}
	if len(d.Authors) > 0 {
		b.Author = d.Authors[0].Name
	}
	if len(d.Publishers) > 0 {
		b.Publisher = d.Publishers[0].Name
	}
	for _, s := range d.Subjects {
		b.Subjects = append(b.Subjects, s.Name)
	}
	if m := yearRE.FindString(d.PublishDate); m != "" {
		b.Year, _ = strconv.Atoi(m)
	}
	return b, nil
}

func (o *OpenLibrary) Search(ctx context.Context, query string) (bookmeta.Book, error) {
	u := fmt.Sprintf("%s/search.json?title=%s&limit=1&fields=title,author_name,cover_i,first_publish_year,isbn",
		o.base, url.QueryEscape(query))
	var raw struct {
		Docs []struct {
			Title      string   `json:"title"`
			AuthorName []string `json:"author_name"`
			CoverI     int      `json:"cover_i"`
			Year       int      `json:"first_publish_year"`
			ISBN       []string `json:"isbn"`
		} `json:"docs"`
	}
	if err := o.getJSON(ctx, u, &raw); err != nil {
		return bookmeta.Book{}, err
	}
	if len(raw.Docs) == 0 {
		return bookmeta.Book{}, nil
	}
	d := raw.Docs[0]
	b := bookmeta.Book{Title: d.Title, Year: d.Year}
	if len(d.AuthorName) > 0 {
		b.Author = d.AuthorName[0]
	}
	if d.CoverI > 0 {
		b.CoverURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", d.CoverI)
	}
	for _, isbn := range d.ISBN {
		if n := bookmeta.NormalizeISBN(isbn); bookmeta.PlausibleISBN13(n) {
			b.ISBN13 = n
			break
		}
	}
	return b, nil
}

func (o *OpenLibrary) getJSON(ctx context.Context, u string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "dungeonbooks-marty/0.1")
	resp, err := o.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("openlibrary: status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
