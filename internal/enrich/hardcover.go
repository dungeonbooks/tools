package enrich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dungeonbooks/tools/internal/bookmeta"
)

// Hardcover is the GraphQL source for rating, genres, series, and description.
// Requires an Authorization token; without one, callers fall back to OpenLibrary.
type Hardcover struct {
	http  *http.Client
	url   string
	token string
}

func NewHardcover(token string, hc *http.Client) *Hardcover {
	return &Hardcover{http: hc, url: "https://api.hardcover.app/v1/graphql", token: token}
}

func (h *Hardcover) Enabled() bool { return h.token != "" }

const searchQuery = `query SearchBooks($query: String!, $limit: Int!) {
  search(query: $query, query_type: "books", per_page: $limit, page: 1, sort: "activities_count:desc") { ids }
}`

const bookQuery = `query GetBook($id: Int!) {
  books_by_pk(id: $id) {
    title subtitle description pages release_year rating ratings_count
    cached_tags slug
    image { url }
    contributions { author { name } }
    editions { isbn_13 }
  }
}`

const editionByISBNQuery = `query EditionByISBN($isbn: String!) {
  editions(where: {isbn_13: {_eq: $isbn}}, limit: 1) { book { id } }
}`

// ByISBN resolves an ISBN-13 to a fully detailed book (Hardcover carries new
// releases that OpenLibrary often lacks).
func (h *Hardcover) ByISBN(ctx context.Context, isbn string) (bookmeta.Book, error) {
	var er struct {
		Editions []struct {
			Book struct {
				ID int `json:"id"`
			} `json:"book"`
		} `json:"editions"`
	}
	if err := h.do(ctx, editionByISBNQuery, map[string]any{"isbn": isbn}, &er); err != nil {
		return bookmeta.Book{}, err
	}
	if len(er.Editions) == 0 {
		return bookmeta.Book{}, nil
	}
	return h.byID(ctx, er.Editions[0].Book.ID)
}

// SearchTop returns the top-ranked book for a query, fully detailed.
func (h *Hardcover) SearchTop(ctx context.Context, query string) (bookmeta.Book, error) {
	var sr struct {
		Search struct {
			IDs []int `json:"ids"`
		} `json:"search"`
	}
	if err := h.do(ctx, searchQuery, map[string]any{"query": query, "limit": 1}, &sr); err != nil {
		return bookmeta.Book{}, err
	}
	if len(sr.Search.IDs) == 0 {
		return bookmeta.Book{}, nil
	}
	return h.byID(ctx, sr.Search.IDs[0])
}

func (h *Hardcover) byID(ctx context.Context, id int) (bookmeta.Book, error) {
	var br struct {
		Book *struct {
			Title        string          `json:"title"`
			Subtitle     string          `json:"subtitle"`
			Description  string          `json:"description"`
			Pages        int             `json:"pages"`
			ReleaseYear  int             `json:"release_year"`
			Rating       float64         `json:"rating"`
			RatingsCount int             `json:"ratings_count"`
			CachedTags   json.RawMessage `json:"cached_tags"`
			Slug         string          `json:"slug"`
			Image        struct {
				URL string `json:"url"`
			} `json:"image"`
			Contributions []struct {
				Author struct {
					Name string `json:"name"`
				} `json:"author"`
			} `json:"contributions"`
			Editions []struct {
				ISBN13 string `json:"isbn_13"`
			} `json:"editions"`
		} `json:"books_by_pk"`
	}
	if err := h.do(ctx, bookQuery, map[string]any{"id": id}, &br); err != nil {
		return bookmeta.Book{}, err
	}
	d := br.Book
	if d == nil {
		return bookmeta.Book{}, nil
	}
	b := bookmeta.Book{
		Title:        d.Title,
		Description:  d.Description,
		PageCount:    d.Pages,
		Year:         d.ReleaseYear,
		Rating:       d.Rating,
		RatingsCount: d.RatingsCount,
		Subjects:     genreTags(d.CachedTags),
	}
	if d.Slug != "" {
		b.HardcoverURL = "https://hardcover.app/books/" + d.Slug
	}
	if d.Image.URL != "" {
		b.CoverURL = d.Image.URL
	}
	if len(d.Contributions) > 0 {
		b.Author = d.Contributions[0].Author.Name
	}
	for _, e := range d.Editions {
		if n := bookmeta.NormalizeISBN(e.ISBN13); bookmeta.PlausibleISBN13(n) {
			b.ISBN13 = n
			break
		}
	}
	return b, nil
}

func (h *Hardcover) do(ctx context.Context, query string, vars map[string]any, out any) error {
	body, _ := json.Marshal(map[string]any{"query": query, "variables": vars})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", h.token)
	resp, err := h.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hardcover: status %d", resp.StatusCode)
	}
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return err
	}
	if len(env.Errors) > 0 {
		return fmt.Errorf("hardcover: %s", env.Errors[0].Message)
	}
	return json.Unmarshal(env.Data, out)
}

func genreTags(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var tags map[string][]struct {
		Tag string `json:"tag"`
	}
	if err := json.Unmarshal(raw, &tags); err != nil {
		return nil
	}
	var out []string
	for _, g := range tags["Genre"] {
		if g.Tag != "" {
			out = append(out, g.Tag)
		}
	}
	return out
}
