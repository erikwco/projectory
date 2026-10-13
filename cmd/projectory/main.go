package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/erikwco/projectory/internal/cli"
)

var version = "1.0.0-dev"

func main() {
	if err := cli.New(version).Execute(); err != nil {
		if !errors.Is(err, cli.ErrCancelled) {
			fmt.Fprintln(os.Stderr, "projectory:", err)
		}
		os.Exit(1)
	}
}
