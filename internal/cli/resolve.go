package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/dungeonbooks/tools/internal/clierr"
	"github.com/dungeonbooks/tools/internal/enrich"
	"github.com/dungeonbooks/tools/internal/platform/config"
	"github.com/dungeonbooks/tools/internal/resolve"
	"github.com/spf13/cobra"
)

func resolveCmd() *cobra.Command {
	var author, isbn string
	c := &cobra.Command{
		Use:   "resolve [title] | resolve --isbn <isbn>",
		Short: "Resolve a title to a verified ISBN-13",
		Long: "Resolve a title to a verified ISBN-13.\n\n" +
			"Pass --author whenever you know it: a bare title mismatches badly.\n\n" +
			"The exit code is the contract:\n" +
			"  0  verified — the ISBN on stdout is this book's\n" +
			"  3  not found — nothing cleared the confidence floor, or no catalogue\n" +
			"     carries the book. There is no ISBN to give; do not invent one\n" +
			"  5  upstream — the lookup itself failed. This says nothing about the\n" +
			"     book. Retry before concluding anything\n\n" +
			"An unverified ISBN is never written to stdout. On a failure the reason\n" +
			"goes to stderr, and stdout carries only the JSON payload (if --json).",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.Join(args, " ")
			if isbn == "" && title == "" {
				return clierr.Usage(errors.New("provide a title or --isbn"))
			}
			cfg := config.Load()
			svc := enrich.New(cfg.HardcoverToken, cfg.GoogleBooksKey)

			var r resolve.Result
			if isbn != "" {
				r = resolve.ISBN(cmd.Context(), svc, isbn)
			} else {
				r = resolve.Title(cmd.Context(), svc, title, author)
			}

			// The payload is written either way — a --json caller still wants the
			// rejected candidate to inspect — but the verdict rides the exit code.
			if err := renderResolved(cmd.OutOrStdout(), r, jsonOut); err != nil {
				return err
			}
			return resolveErr(r)
		},
	}
	c.Flags().StringVar(&author, "author", "", "author (strongly recommended; prevents confident wrong matches)")
	c.Flags().StringVar(&isbn, "isbn", "", "verify a known ISBN-13 instead of resolving a title")
	return c
}

// resolveErr types an unverified result so the exit code says which kind of
// failure it was. The distinction is the whole point: a book no catalogue
// carries (3) and a provider that fell over (5) are opposite facts, and a caller
// that cannot tell them apart will report an outage as if the book did not exist.
func resolveErr(r resolve.Result) error {
	if r.Verified {
		return nil
	}
	msg := fmt.Sprintf("%s (query %q)", r.Reason, r.Query)
	if r.Retryable {
		return clierr.Upstream(errors.New(msg))
	}
	if r.ISBN13 != "" {
		msg += fmt.Sprintf("; the best guess was %s (%s), which failed verification — do not use it as this book's ISBN",
			r.ISBN13, r.Describe())
	}
	return clierr.NotFound(errors.New(msg))
}

// renderResolved writes the payload only. The reason a lookup failed travels as
// the returned error, so it is reported once, by main, rather than printed here
// and again there.
func renderResolved(out io.Writer, r resolve.Result, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	if !r.Verified {
		return nil
	}
	_, err := fmt.Fprintf(out, "%s  %s  [confidence %.2f]\n", r.ISBN13, r.Describe(), r.Confidence)
	return err
}
