package main

import (
	"os"

	"github.com/atakanatali/contextify/internal/cli"
)

var version = "dev" // set via ldflags at build time

func main() {
	if err := cli.Execute(version); err != nil {
		os.Exit(1)
	}
}
