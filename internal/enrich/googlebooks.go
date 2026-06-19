package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/dungeonbooks/tools/internal/bookmeta"
)

// GoogleBooks is a broad, free source; better new-release coverage than Open
// Library, thinner reader data than Hardcover. Key is optional (keyless is throttled).
type GoogleBooks struct {
	http *http.Client
	key  string
	base string
}

func NewGoogleBooks(key string, hc *http.Client) *GoogleBooks {
	return &GoogleBooks{http: hc, key: key, base: "https://www.googleapis.com/books/v1/volumes"}
}

func (g *GoogleBooks) ByISBN(ctx context.Context, isbn string) (bookmeta.Book, error) {
	return g.query(ctx, "isbn:"+isbn)
}

func (g *GoogleBooks) Search(ctx context.Context, query string) (bookmeta.Book, error) {
	return g.query(ctx, query)
}

func (g *GoogleBooks) query(ctx context.Context, q string) (bookmeta.Book, error) {
	u := fmt.Sprintf("%s?q=%s&country=US&maxResults=1", g.base, url.QueryEscape(q))
	if g.key != "" {
		u += "&key=" + url.QueryEscape(g.key)
	}
	var raw struct {
		Items []struct {
			VolumeInfo struct {
				Title         string   `json:"title"`
				Authors       []string `json:"authors"`
				Description   string   `json:"description"`
				Publisher     string   `json:"publisher"`
				PublishedDate string   `json:"publishedDate"`
				PageCount     int      `json:"pageCount"`
				Categories    []string `json:"categories"`
				AverageRating float64  `json:"averageRating"`
				RatingsCount  int      `json:"ratingsCount"`
				ImageLinks    struct {
					Thumbnail string `json:"thumbnail"`
				} `json:"imageLinks"`
				IndustryIdentifiers []struct {
					Type       string `json:"type"`
					Identifier string `json:"identifier"`
				} `json:"industryIdentifiers"`
			} `json:"volumeInfo"`
		} `json:"items"`
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return bookmeta.Book{}, err
	}
	resp, err := g.http.Do(req)
	if err != nil {
		return bookmeta.Book{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return bookmeta.Book{}, fmt.Errorf("googlebooks: status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return bookmeta.Book{}, err
	}
	if len(raw.Items) == 0 {
		return bookmeta.Book{}, nil
	}
	v := raw.Items[0].VolumeInfo
	b := bookmeta.Book{
		Title:        v.Title,
		Description:  v.Description,
		Publisher:    v.Publisher,
		PageCount:    v.PageCount,
		Subjects:     v.Categories,
		Rating:       v.AverageRating,
		RatingsCount: v.RatingsCount,
		CoverURL:     v.ImageLinks.Thumbnail,
	}
	if len(v.Authors) > 0 {
		b.Author = v.Authors[0]
	}
	if m := yearRE.FindString(v.PublishedDate); m != "" {
		b.Year, _ = strconv.Atoi(m)
	}
	for _, id := range v.IndustryIdentifiers {
		if id.Type == "ISBN_13" {
			if n := bookmeta.NormalizeISBN(id.Identifier); bookmeta.PlausibleISBN13(n) {
				b.ISBN13 = n
			}
		}
	}
	return b, nil
}
