package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dungeonbooks/tools/internal/bookmeta"
)

const exaEndpoint = "https://api.exa.ai/search"

type ExaClient struct {
	apiKey string
	http   *http.Client
}

func NewExaClient(apiKey string, hc *http.Client) *ExaClient {
	return &ExaClient{apiKey: apiKey, http: hc}
}

var bookSchema = map[string]any{
	"type":     "object",
	"required": []string{"books"},
	"properties": map[string]any{
		"books": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":     "object",
				"required": []string{"title"},
				"properties": map[string]any{
					"title":        map[string]any{"type": "string"},
					"author":       map[string]any{"type": "string"},
					"why_trending": map[string]any{"type": "string"},
					"isbn":         map[string]any{"type": "string"},
					"source_url":   map[string]any{"type": "string"},
				},
			},
		},
	},
}

func (c *ExaClient) Search(ctx context.Context, query string, numResults int) ([]bookmeta.Book, error) {
	body, _ := json.Marshal(map[string]any{
		"query":        query,
		"type":         "auto",
		"numResults":   numResults,
		"contents":     map[string]any{"highlights": true},
		"outputSchema": bookSchema,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, exaEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exa: status %d", resp.StatusCode)
	}

	var out struct {
		Output struct {
			Content struct {
				Books []struct {
					Title       string `json:"title"`
					Author      string `json:"author"`
					WhyTrending string `json:"why_trending"`
					ISBN        string `json:"isbn"`
					SourceURL   string `json:"source_url"`
				} `json:"books"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	books := make([]bookmeta.Book, 0, len(out.Output.Content.Books))
	for _, b := range out.Output.Content.Books {
		isbn := bookmeta.NormalizeISBN(b.ISBN)
		if !bookmeta.PlausibleISBN13(isbn) {
			isbn = ""
		}
		books = append(books, bookmeta.Book{
			ISBN13:      isbn,
			Title:       b.Title,
			Author:      b.Author,
			WhyTrending: b.WhyTrending,
			SourceURL:   b.SourceURL,
		})
	}
	return books, nil
}
