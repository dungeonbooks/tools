package main

import (
	"fmt"
	"os"

	"github.com/dungeonbooks/tools/internal/cli"
	"github.com/dungeonbooks/tools/internal/clierr"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "marty:", err)
		os.Exit(clierr.Code(err))
	}
}
