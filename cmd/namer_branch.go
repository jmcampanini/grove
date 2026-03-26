package cmd

import (
	"fmt"
	"io"

	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/spf13/cobra"
)

var namerBranchCmd = &cobra.Command{
	Use:   "branch <phrase>",
	Short: "Generate a branch name from a phrase",
	Args:  cobra.ExactArgs(1),
	RunE:  runNamerBranch,
}

func init() {
	namerCmd.AddCommand(namerBranchCmd)
}

func runNamerBranch(cmd *cobra.Command, args []string) error {
	cfg, err := loadNamingConfig()
	if err != nil {
		return err
	}

	ctx := &namerContext{
		namer: naming.NewLocalBranchNamer(cfg.LocalBranch, cfg.Slugify),
	}

	return executeNamerBranch(cmd.OutOrStdout(), ctx, args[0])
}

func executeNamerBranch(w io.Writer, ctx *namerContext, phrase string) error {
	name, err := generateBranchName(ctx, phrase)
	if err != nil {
		return err
	}

	if _, err = fmt.Fprintln(w, name); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}
