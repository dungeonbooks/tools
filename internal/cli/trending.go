package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dungeonbooks/tools/internal/discover"
	"github.com/dungeonbooks/tools/internal/platform/config"
	"github.com/spf13/cobra"
)

func trendingCmd() *cobra.Command {
	var source, typ string
	var count int
	var refresh, noCache bool
	c := &cobra.Command{
		Use:   "trending [query...]",
		Short: "Discover trending books from web buzz",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			cfg := config.Load()
			svc := discover.New(cfg.ExaAPIKey)

			if !noCache {
				if cache, err := openTrendingCache(cfg); err == nil {
					defer cache.Close()
					svc.WithCache(cache)
					if exa, ok := exaProvider(svc); ok {
						exa.OnSpend(func(cost float64) {
							if err := cache.RecordSpend(cost); err != nil {
								fmt.Fprintf(os.Stderr, "marty: record spend: %v\n", err)
							}
						})
					}
					defer printUsage(cache, cmd)
				} else {
					fmt.Fprintf(os.Stderr, "marty: cache disabled: %v\n", err)
				}
			}

			cs, err := svc.Trending(cmd.Context(), query, source, typ, count, refresh)
			if err != nil {
				return err
			}
			return renderTrending(cmd.OutOrStdout(), cs, jsonOut)
		},
	}
	c.Flags().StringVar(&source, "source", "", "force one source: fake or exa (default: first available)")
	c.Flags().StringVar(&typ, "type", "auto", "search mode: auto, neural, or keyword")
	c.Flags().IntVar(&count, "count", 10, "max results")
	c.Flags().BoolVar(&refresh, "refresh", false, "bypass the cache for this call")
	c.Flags().BoolVar(&noCache, "no-cache", false, "disable the local cache entirely")
	return c
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

func printUsage(cache *discover.Cache, cmd *cobra.Command) {
	dollars, calls, err := cache.Usage()
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Exa spend: $%.2f across %d call(s)\n", dollars, calls)
}

func exaProvider(svc *discover.Service) (*discover.Exa, bool) {
	for _, p := range svc.Providers() {
		if e, ok := p.(*discover.Exa); ok {
			return e, true
		}
	}
	return nil, false
}
