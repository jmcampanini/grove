// Grove manages git worktrees: one directory per branch under a configurable
// root. The command tree lives in internal/cli.
package main

import (
	"os"

	"github.com/jmcampanini/grove/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
