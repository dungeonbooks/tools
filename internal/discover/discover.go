package discover

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dungeonbooks/tools/internal/clierr"
)

const (
	SourceFake = "fake"
	SourceExa  = "exa"

	TypeAuto    = "auto"
	TypeNeural  = "neural"
	TypeKeyword = "keyword"

	defaultCount = 10
)

// searchTypes is the set of valid --type values, used both to validate input and
// to render a helpful "available: ..." error, mirroring how source is checked.
var searchTypes = []string{TypeAuto, TypeNeural, TypeKeyword}

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

// ISBNResolver turns a title/author into an ISBN-13. Web-buzz candidates rarely
// carry ISBNs, so the service chains this lookup after discovery. It lives as an
// interface here so discover stays free of metadata-provider deps; the cli layer
// implements it with the enrich service.
type ISBNResolver interface {
	ResolveISBN(ctx context.Context, title, author string) (string, error)
}

// resolveWorkers bounds concurrent ISBN lookups so a large --count doesn't fan
// out a burst of requests at the metadata providers.
const resolveWorkers = 5

type Service struct {
	providers []Provider
	cache     *Cache
	resolver  ISBNResolver
}

func NewService(providers ...Provider) *Service {
	return &Service{providers: providers}
}

// WithCache attaches a local cache so paid provider results are reused within
// the TTL; --refresh bypasses it. Fake results are cached too (harmless) so
// the cache key includes source to keep entries distinct.
func (s *Service) WithCache(c *Cache) *Service {
	s.cache = c
	return s
}

// WithResolver attaches an ISBN resolver. Fresh provider results have their
// ISBNs filled in before caching, so the (free) lookups run once per cache
// entry rather than on every call.
func (s *Service) WithResolver(r ISBNResolver) *Service {
	s.resolver = r
	return s
}

// New builds the default service with the real Exa provider plus the offline
// Fake fixture. Auto-pick uses Exa (when a key is set) and never silently falls
// back to Fake; Fake is selectable only via an explicit --source fake.
func New(exaKey string) *Service {
	hc := &http.Client{Timeout: 30 * time.Second}
	exa := NewExa(exaKey, hc)
	return NewService(exa, NewFake())
}

// Trending returns candidates plus the name of the provider that served them, so
// callers can label provenance (notably the Fake fixture, which must never be
// mistaken for real results).
func (s *Service) Trending(ctx context.Context, query, source, typ string, count int, refresh bool) ([]Candidate, string, error) {
	if typ == "" {
		typ = TypeAuto
	}
	if !validType(typ) {
		return nil, "", clierr.Usage(fmt.Errorf("unknown type %q (available: %s)", typ, strings.Join(searchTypes, ", ")))
	}
	if count <= 0 {
		count = defaultCount
	}
	p, err := s.pick(source)
	if err != nil {
		return nil, "", err
	}
	src := p.Name()
	key := cacheKey(src, typ, query, count)
	if s.cache != nil && !refresh {
		if cs, ok, err := s.cache.Get(key); err != nil {
			return nil, src, err
		} else if ok {
			return cs, src, nil
		}
	}
	cs, err := p.Trending(ctx, query, typ, count)
	if err != nil {
		return nil, src, err
	}
	if s.resolver != nil {
		s.resolveISBNs(ctx, cs)
	}
	if s.cache != nil {
		if err := s.cache.Put(key, cs); err != nil {
			return nil, src, err
		}
	}
	return cs, src, nil
}

func validType(typ string) bool {
	for _, t := range searchTypes {
		if t == typ {
			return true
		}
	}
	return false
}

// resolveISBNs fills empty ISBNs in place, concurrently and bounded. Lookup
// failures are left silent: an unresolved ISBN just stays empty, which the
// renderer already handles, so a flaky metadata provider never fails discovery.
func (s *Service) resolveISBNs(ctx context.Context, cs []Candidate) {
	sem := make(chan struct{}, resolveWorkers)
	var wg sync.WaitGroup
	for i := range cs {
		if cs[i].ISBN13 != "" || cs[i].Title == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(c *Candidate) {
			defer wg.Done()
			defer func() { <-sem }()
			if isbn, err := s.resolver.ResolveISBN(ctx, c.Title, c.Author); err == nil {
				c.ISBN13 = isbn
			}
		}(&cs[i])
	}
	wg.Wait()
}

func (s *Service) pick(source string) (Provider, error) {
	if source != "" {
		for _, p := range s.providers {
			if p.Name() == source {
				if !p.Enabled() {
					return nil, clierr.Auth(fmt.Errorf("%s source unavailable", source))
				}
				return p, nil
			}
		}
		return nil, clierr.Usage(fmt.Errorf("unknown source %q (available: %s)", source, strings.Join(s.names(), ", ")))
	}
	// Auto-pick never falls back to the Fake fixture: it returns invented books,
	// which must not masquerade as real discovery. Fake is reachable only via an
	// explicit --source fake.
	for _, p := range s.providers {
		if p.Name() == SourceFake {
			continue
		}
		if p.Enabled() {
			return p, nil
		}
	}
	return nil, clierr.Auth(errors.New("no discovery provider available: set EXA_API_KEY or pass --source fake"))
}

func (s *Service) Providers() []Provider { return s.providers }

func (s *Service) names() []string {
	out := make([]string, len(s.providers))
	for i, p := range s.providers {
		out[i] = p.Name()
	}
	return out
}
