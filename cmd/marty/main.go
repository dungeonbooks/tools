package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/dungeonbooks/tools/internal/cli"
	"github.com/dungeonbooks/tools/internal/clierr"
)

func main() {
	err := cli.Execute()
	switch {
	case err == nil:
	case errors.Is(err, cli.ErrUnverified):
		// The command already reported why. Exit 1 is the whole message.
		os.Exit(1)
	default:
		fmt.Fprintln(os.Stderr, "marty:", err)
		os.Exit(clierr.Code(err))
	}
}
