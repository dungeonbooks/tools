package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/dungeonbooks/tools/internal/bookmeta"
)

// ISBNResolver turns a title+author into an ISBN-13 via Open Library search.
type ISBNResolver struct {
	http *http.Client
}

func NewISBNResolver(hc *http.Client) *ISBNResolver {
	return &ISBNResolver{http: hc}
}

// Resolve returns a best-effort ISBN-13, or "" if none found. A non-nil error
// means the lookup itself failed, not that the book was missing.
func (r *ISBNResolver) Resolve(ctx context.Context, title, author string) (string, error) {
	if strings.TrimSpace(title) == "" {
		return "", nil
	}
	q := url.Values{"title": {title}, "fields": {"isbn"}, "limit": {"1"}}
	if author != "" {
		q.Set("author", author)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://openlibrary.org/search.json?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "dungeonbooks-tools/0.1")

	resp, err := r.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openlibrary: status %d", resp.StatusCode)
	}

	var out struct {
		Docs []struct {
			ISBN []string `json:"isbn"`
		} `json:"docs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	for _, d := range out.Docs {
		for _, isbn := range d.ISBN {
			if n := bookmeta.NormalizeISBN(isbn); bookmeta.PlausibleISBN13(n) {
				return n, nil
			}
		}
	}
	return "", nil
}
