package cli

import (
	"io"
	"os"

	"github.com/dungeonbooks/tools/internal/clierr"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var jsonOut bool

func Execute() error {
	root := &cobra.Command{
		Use:           "marty",
		Short:         "Marty — the bookseller's wizard",
		SilenceUsage:  true,
		SilenceErrors: true,
		// Agent-native default: emit JSON when stdout is piped, human tables when
		// it's a terminal. An explicit --json / --json=false always wins.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if !cmd.Flags().Changed("json") {
				jsonOut = !isTerminalWriter(cmd.OutOrStdout())
			}
			return nil
		},
	}
	// Flag-parse failures are usage errors (exit 2), not generic failures.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return clierr.Usage(err)
	})
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "output JSON (default: auto — JSON when piped, tables on a terminal)")
	root.AddCommand(bookCmd())
	root.AddCommand(trendingCmd())
	root.AddCommand(resolveCmd())
	root.AddCommand(mcpCmd())
	return root.Execute()
}

// isTerminalWriter reports whether w is an interactive terminal. Anything that
// isn't an *os.File (a pipe, a test buffer) counts as non-interactive.
func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && isatty.IsTerminal(f.Fd())
}

// usageArgs tags an argument-count failure as a usage error (exit 2).
func usageArgs(fn cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		return clierr.Usage(fn(cmd, args))
	}
}
