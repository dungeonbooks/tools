// Package discovery finds trending books via Exa and resolves each to an ISBN-13.
package discovery

import (
	"context"
	"log/slog"
	"sync"

	"github.com/dungeonbooks/tools/internal/bookmeta"
)

type Service struct {
	exa      *ExaClient
	resolver *ISBNResolver
	log      *slog.Logger
}

func NewService(exa *ExaClient, resolver *ISBNResolver, log *slog.Logger) *Service {
	return &Service{exa: exa, resolver: resolver, log: log}
}

// Discover searches, then fills in ISBNs the search didn't provide. A failed
// resolution is logged and left blank, never fatal.
func (s *Service) Discover(ctx context.Context, query string, numResults int) ([]bookmeta.Book, error) {
	books, err := s.exa.Search(ctx, query, numResults)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	for i := range books {
		if books[i].ISBN13 != "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			isbn, err := s.resolver.Resolve(ctx, books[i].Title, books[i].Author)
			if err != nil {
				s.log.Warn("isbn resolve failed", "title", books[i].Title, "err", err)
				return
			}
			books[i].ISBN13 = isbn
		}(i)
	}
	wg.Wait()
	return books, nil
}
