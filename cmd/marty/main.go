package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/dungeonbooks/tools/internal/cli"
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
		os.Exit(1)
	}
}
