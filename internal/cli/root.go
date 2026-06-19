package cli

import (
	"github.com/spf13/cobra"
)

var jsonOut bool

func Execute() error {
	root := &cobra.Command{
		Use:           "marty",
		Short:         "Marty — the bookseller's wizard",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "output JSON")
	root.AddCommand(bookCmd())
	return root.Execute()
}
