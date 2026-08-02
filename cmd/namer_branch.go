package cmd

import (
	"fmt"
	"io"

	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/spf13/cobra"
)

func newNamerBranchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "branch <phrase>",
		Short: "Generate a branch name from a phrase",
		Args:  cobra.ExactArgs(1),
		RunE:  runNamerBranch,
	}
}

func runNamerBranch(cmd *cobra.Command, args []string) error {
	cfg, err := loadNamingConfig(cmd)
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
