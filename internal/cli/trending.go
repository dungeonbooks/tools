package cli

import (
	"github.com/dungeonbooks/tools/internal/discover"
	"github.com/dungeonbooks/tools/internal/platform/config"
	"github.com/spf13/cobra"
)

func trendingCmd() *cobra.Command {
	var source, typ string
	var count int
	var refresh bool
	c := &cobra.Command{
		Use:   "trending [query]",
		Short: "Discover trending books from web buzz",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = args[0]
			}
			cfg := config.Load()
			svc := discover.New(cfg.ExaAPIKey)
			cs, err := svc.Trending(cmd.Context(), query, source, typ, count)
			if err != nil {
				return err
			}
			return renderTrending(cmd.OutOrStdout(), cs, jsonOut)
		},
	}
	c.Flags().StringVar(&source, "source", "", "force one source: fake (exa lands in the next change; default: auto)")
	c.Flags().StringVar(&typ, "type", "auto", "search mode: auto, neural, or keyword")
	c.Flags().IntVar(&count, "count", 10, "max results")
	c.Flags().BoolVar(&refresh, "refresh", false, "bypass the cache (no-op until caching lands)")
	return c
}
