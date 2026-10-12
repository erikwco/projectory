package main

import (
	"fmt"
	"os"

	"github.com/erikwco/projectory/internal/cli"
)

var version = "1.0.0-dev"

func main() {
	if err := cli.New(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "projectory:", err)
		os.Exit(1)
	}
}
