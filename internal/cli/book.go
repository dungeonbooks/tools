package cli

import (
	"fmt"
	"strings"

	"github.com/dungeonbooks/tools/internal/enrich"
	"github.com/dungeonbooks/tools/internal/platform/config"
	"github.com/spf13/cobra"
)

func bookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "book <title|isbn>",
		Short: "Look up a book with rich metadata",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			cfg := config.Load()
			svc := enrich.New(cfg.HardcoverToken, cfg.GoogleBooksKey)
			b, err := svc.Book(cmd.Context(), query)
			if err != nil {
				return err
			}
			if b.Title == "" {
				return fmt.Errorf("no book found for %q", query)
			}
			return renderBook(cmd.OutOrStdout(), b, jsonOut)
		},
	}
}
