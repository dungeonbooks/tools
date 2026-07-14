package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/dungeonbooks/tools/internal/enrich"
	"github.com/dungeonbooks/tools/internal/platform/config"
	"github.com/dungeonbooks/tools/internal/resolve"
	"github.com/spf13/cobra"
)

// ErrUnverified reports that no match cleared the confidence floor. It drives a
// non-zero exit without a second error message, since the command has already
// written the result and the reason it failed verification.
var ErrUnverified = errors.New("unverified match")

func resolveCmd() *cobra.Command {
	var author, isbn string
	c := &cobra.Command{
		Use:   "resolve [title] | resolve --isbn <isbn>",
		Short: "Resolve a title to a verified ISBN-13",
		Long: "Resolve a title to a verified ISBN-13.\n\n" +
			"Pass --author whenever you know it: a bare title mismatches badly.\n" +
			"Exits 0 on a verified match and 1 when nothing clears the confidence\n" +
			"floor, so an unverified ISBN is never mistaken for an answer. With\n" +
			"--json the result goes to stdout either way; the human rendering sends\n" +
			"an unverified result to stderr.",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.Join(args, " ")
			if isbn == "" && title == "" {
				cmd.SilenceUsage = false
				return errors.New("provide a title or --isbn")
			}
			cfg := config.Load()
			svc := enrich.New(cfg.HardcoverToken, cfg.GoogleBooksKey)

			var r resolve.Result
			if isbn != "" {
				r = resolve.ISBN(cmd.Context(), svc, isbn)
			} else {
				r = resolve.Title(cmd.Context(), svc, title, author)
			}

			if err := renderResolved(cmd.OutOrStdout(), cmd.ErrOrStderr(), r, jsonOut); err != nil {
				return err
			}
			if !r.Verified {
				return ErrUnverified
			}
			return nil
		},
	}
	c.Flags().StringVar(&author, "author", "", "author (strongly recommended; prevents confident wrong matches)")
	c.Flags().StringVar(&isbn, "isbn", "", "verify a known ISBN-13 instead of resolving a title")
	return c
}

func renderResolved(out, errOut io.Writer, r resolve.Result, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	if r.Verified {
		_, err := fmt.Fprintf(out, "%s  %s  [confidence %.2f]\n", r.ISBN13, r.Describe(), r.Confidence)
		return err
	}
	label := "UNVERIFIED"
	if r.Retryable {
		label = "LOOKUP FAILED"
	}
	if _, err := fmt.Fprintf(errOut, "%s: %s  query=%q\n", label, r.Reason, r.Query); err != nil {
		return err
	}
	if r.ISBN13 != "" {
		_, err := fmt.Fprintf(errOut, "  best guess (do not trust): %s  %s\n", r.ISBN13, r.Describe())
		return err
	}
	return nil
}
