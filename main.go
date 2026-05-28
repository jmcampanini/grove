package main

import (
	"fmt"
	"os"

	"github.com/jmcampanini/grove-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "grove: error: %v\n", err)
		os.Exit(1)
	}
}
