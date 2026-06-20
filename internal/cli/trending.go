package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dungeonbooks/tools/internal/discover"
	"github.com/dungeonbooks/tools/internal/enrich"
	"github.com/dungeonbooks/tools/internal/platform/config"
	"github.com/spf13/cobra"
)

func trendingCmd() *cobra.Command {
	var source, typ string
	var count int
	var refresh, noCache, noResolve bool
	c := &cobra.Command{
		Use:   "trending [query...]",
		Short: "Discover trending books from web buzz",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			cfg := config.Load()
			svc := discover.New(cfg.ExaAPIKey)

			if !noResolve {
				svc.WithResolver(isbnResolver{enrich.New(cfg.HardcoverToken, cfg.GoogleBooksKey)})
			}

			// callSpend captures only what Exa billed for this invocation, so the
			// cost line reflects this call, not the lifetime running total.
			var callSpend float64
			var cache *discover.Cache
			if !noCache {
				if c, err := openTrendingCache(cfg); err == nil {
					cache = c
					defer cache.Close()
					svc.WithCache(cache)
				} else {
					fmt.Fprintf(os.Stderr, "marty: cache disabled: %v\n", err)
				}
			}
			if exa, ok := exaProvider(svc); ok {
				exa.OnSpend(func(cost float64) {
					callSpend += cost
					if cache != nil {
						if err := cache.RecordSpend(cost); err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "marty: record spend: %v\n", err)
						}
					}
				})
			}

			cs, src, err := svc.Trending(cmd.Context(), query, source, typ, count, refresh)
			if err != nil {
				return err
			}
			if err := renderTrending(cmd.OutOrStdout(), cs, src, jsonOut); err != nil {
				return err
			}
			reportSpend(cmd.ErrOrStderr(), cache, callSpend)
			return nil
		},
	}
	c.Flags().StringVar(&source, "source", "", "force one source: exa or fake (default: exa when EXA_API_KEY is set)")
	c.Flags().StringVar(&typ, "type", "auto", "search mode: auto, neural, or keyword")
	c.Flags().IntVar(&count, "count", 10, "max results")
	c.Flags().BoolVar(&refresh, "refresh", false, "bypass the cache for this call")
	c.Flags().BoolVar(&noCache, "no-cache", false, "disable the local cache entirely")
	c.Flags().BoolVar(&noResolve, "no-resolve", false, "skip ISBN resolution via metadata lookup")
	return c
}

// isbnResolver adapts the enrich service to discover.ISBNResolver, chaining a
// title/author lookup onto each web-buzz candidate that arrives without an ISBN.
type isbnResolver struct {
	svc *enrich.Service
}

func (r isbnResolver) ResolveISBN(ctx context.Context, title, author string) (string, error) {
	return r.svc.ISBN(ctx, title, author)
}

func openTrendingCache(cfg config.Config) (*discover.Cache, error) {
	path := cfg.CachePath
	if path == "" {
		dir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(dir, ".local", "share", "marty", "cache.db")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return discover.OpenCache(path)
}

// reportSpend prints the cost of paid Exa work done in this call, and only then.
// A cache hit, the Fake source, or a failure before any paid call all leave
// callSpend at zero, so nothing prints. When a cache is present the lifetime
// total is appended for credit-burn awareness.
func reportSpend(w io.Writer, cache *discover.Cache, callSpend float64) {
	if callSpend <= 0 {
		return
	}
	line := fmt.Sprintf("Exa: $%.4f this call", callSpend)
	if cache != nil {
		if dollars, calls, err := cache.Usage(); err == nil {
			line += fmt.Sprintf(" · $%.4f total across %d call(s)", dollars, calls)
		}
	}
	fmt.Fprintln(w, line)
}

func exaProvider(svc *discover.Service) (*discover.Exa, bool) {
	for _, p := range svc.Providers() {
		if e, ok := p.(*discover.Exa); ok {
			return e, true
		}
	}
	return nil, false
}
