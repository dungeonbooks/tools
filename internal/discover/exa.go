package discover

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	exaBase      = "https://api.exa.ai"
	defaultQuery = "Which books are trending on BookTok and Reddit right now? " +
		"Focus on recently released or upcoming titles with real reader buzz."
)

// bookSchema is the structured-output contract Exa is asked to fill per result.
// ISBN is requested but web-buzz pages rarely carry it, so resolution is a
// separate step (land later); Exa returning it empty is expected.
var bookSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"books": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":        map[string]any{"type": "string"},
					"author":       map[string]any{"type": "string"},
					"why_trending": map[string]any{"type": "string"},
					"source_url":   map[string]any{"type": "string"},
					"isbn":         map[string]any{"type": "string"},
				},
				"required": []string{"title", "why_trending", "source_url"},
			},
		},
	},
	"required": []string{"books"},
}

// Exa is the real discovery provider. Each Trending call is a paid /answer
// request, so callers should cache results (see Service).
type Exa struct {
	key     string
	http    *http.Client
	base    string
	onSpend func(cost float64) // optional hook for the usage counter
}

func NewExa(key string, hc *http.Client) *Exa {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &Exa{key: key, http: hc, base: exaBase}
}

// OnSpend registers a hook invoked with costDollars.total after each paid call.
func (e *Exa) OnSpend(fn func(cost float64)) { e.onSpend = fn }

func (e *Exa) Name() string  { return SourceExa }
func (e *Exa) Enabled() bool { return e.key != "" }

func (e *Exa) Trending(ctx context.Context, query, typ string, count int) ([]Candidate, error) {
	if !e.Enabled() {
		return nil, fmt.Errorf("exa source unavailable: set EXA_API_KEY")
	}
	if query == "" {
		query = defaultQuery
	}
	if typ == "" {
		typ = TypeAuto
	}
	if count <= 0 {
		count = defaultCount
	}
	body, err := json.Marshal(map[string]any{
		"query":        query,
		"type":         typ,
		"numResults":   count,
		"outputSchema": bookSchema,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.base+"/answer", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", e.key)
	req.Header.Set("content-type", "application/json")
	resp, err := e.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exa: status %s", resp.Status)
	}
	var raw struct {
		Answer struct {
			Books []struct {
				Title       string `json:"title"`
				Author      string `json:"author"`
				WhyTrending string `json:"why_trending"`
				SourceURL   string `json:"source_url"`
				ISBN        string `json:"isbn"`
			} `json:"books"`
		} `json:"answer"`
		CostDollars struct {
			Total float64 `json:"total"`
		} `json:"costDollars"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if e.onSpend != nil && raw.CostDollars.Total > 0 {
		e.onSpend(raw.CostDollars.Total)
	}
	now := time.Now()
	out := make([]Candidate, 0, len(raw.Answer.Books))
	for _, b := range raw.Answer.Books {
		if b.Title == "" {
			continue
		}
		out = append(out, Candidate{
			Title:        b.Title,
			Author:       b.Author,
			ISBN13:       cleanISBN(b.ISBN),
			WhyTrending:  b.WhyTrending,
			SourceURL:    b.SourceURL,
			DiscoveredAt: now,
		})
		if len(out) >= count {
			break
		}
	}
	return out, nil
}

// cleanISBN drops placeholder text Exa emits when a page carries no ISBN.
func cleanISBN(s string) string {
	switch s {
	case "", "N/A", "n/a", "NA", "null":
		return ""
	}
	return s
}
