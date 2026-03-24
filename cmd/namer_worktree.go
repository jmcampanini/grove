package cmd

import (
	"fmt"
	"io"

	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/spf13/cobra"
)

var namerWorktreeCmd = &cobra.Command{
	Use:   "worktree <phrase>",
	Short: "Generate a worktree directory name from a phrase",
	Args:  cobra.ExactArgs(1),
	RunE:  runNamerWorktree,
}

func init() {
	namerCmd.AddCommand(namerWorktreeCmd)
}

func runNamerWorktree(cmd *cobra.Command, args []string) error {
	cfg, err := loadNamingConfig()
	if err != nil {
		return err
	}

	ctx := &namerContext{
		namer: naming.NewLocalBranchNamer(cfg.LocalBranch, cfg.Slugify),
	}

	return executeNamerWorktree(cmd.OutOrStdout(), ctx, args[0])
}

func executeNamerWorktree(w io.Writer, ctx *namerContext, phrase string) error {
	branchName, err := generateBranchName(ctx, phrase)
	if err != nil {
		return err
	}

	name := ctx.namer.GenerateWorktreeName(branchName)

	if _, err = fmt.Fprintln(w, name); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}
