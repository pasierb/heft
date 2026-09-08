package main

import (
	"fmt"
	"os"

	"github.com/pasierb/heft/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.New(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
