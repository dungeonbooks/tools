package discover

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	SourceFake = "fake"
	SourceExa  = "exa"

	TypeAuto    = "auto"
	TypeNeural  = "neural"
	TypeKeyword = "keyword"

	defaultCount = 10
)

// Candidate is a trending book surfaced by discovery. ISBN13 is resolved later
// (web-buzz results rarely carry ISBNs), so it stays empty here.
type Candidate struct {
	Title        string    `json:"title"`
	Author       string    `json:"author,omitempty"`
	Publisher    string    `json:"publisher,omitempty"`
	ISBN13       string    `json:"isbn13,omitempty"`
	WhyTrending  string    `json:"why_trending,omitempty"`
	SourceURL    string    `json:"source_url,omitempty"`
	DiscoveredAt time.Time `json:"discovered_at,omitempty"`
}

type Provider interface {
	Name() string
	Enabled() bool
	Trending(ctx context.Context, query, typ string, count int) ([]Candidate, error)
}

type Service struct {
	providers []Provider
}

func NewService(providers ...Provider) *Service {
	return &Service{providers: providers}
}

// New builds the default service. The Exa provider lands in a later change, so
// for now the service always uses Fake (offline dev/tests); exaKey is accepted
// to keep the signature stable across that change.
func New(exaKey string) *Service {
	return NewService(NewFake())
}

func (s *Service) Trending(ctx context.Context, query, source, typ string, count int) ([]Candidate, error) {
	if typ == "" {
		typ = TypeAuto
	}
	if count <= 0 {
		count = defaultCount
	}
	if source != "" {
		for _, p := range s.providers {
			if p.Name() == source {
				if !p.Enabled() {
					return nil, fmt.Errorf("%s source unavailable", source)
				}
				return p.Trending(ctx, query, typ, count)
			}
		}
		return nil, fmt.Errorf("unknown source %q (available: %s)", source, strings.Join(s.names(), ", "))
	}
	for _, p := range s.providers {
		if p.Enabled() {
			return p.Trending(ctx, query, typ, count)
		}
	}
	return nil, errors.New("no discovery provider available")
}

func (s *Service) names() []string {
	out := make([]string, len(s.providers))
	for i, p := range s.providers {
		out[i] = p.Name()
	}
	return out
}
