package main

import (
	"fmt"
	"os"

	"github.com/dungeonbooks/tools/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "marty:", err)
		os.Exit(1)
	}
}
