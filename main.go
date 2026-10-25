package main

import (
	"os"

	"github.com/jmcampanini/grove/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
