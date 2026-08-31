package cmd

import (
	"fmt"
	"io"

	"github.com/jmcampanini/grove/internal/naming"
	"github.com/spf13/cobra"
)

func newNamerWorktreeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "worktree <phrase>",
		Short: "Generate a worktree directory name from a phrase",
		Long: `Print the worktree directory name grove create would use for the phrase.
The branch name is generated first (see grove namer branch); its slug, with
the first matching naming.strip_prefixes entry removed, is then rendered
through local_branch.worktree_template as {{.BranchSlug}} and capped at
naming.max_length. The global --worktree-template flag replaces the
template for one invocation.`,
		Args: cobra.ExactArgs(1),
		RunE: runNamerWorktree,
	}
}

func runNamerWorktree(cmd *cobra.Command, args []string) error {
	cfg, err := loadNamingConfig(cmd)
	if err != nil {
		return err
	}

	namer, err := naming.NewLocalBranchNamer(cfg.LocalBranch, cfg.Naming)
	if err != nil {
		return fmt.Errorf("failed to initialize local branch namer: %w", err)
	}

	ctx := &namerContext{namer: namer}
	return executeNamerWorktree(cmd.OutOrStdout(), ctx, args[0])
}

func executeNamerWorktree(w io.Writer, ctx *namerContext, phrase string) error {
	branchName, err := generateBranchName(ctx, phrase)
	if err != nil {
		return err
	}

	name, err := ctx.namer.GenerateWorktreeName(branchName)
	if err != nil {
		return fmt.Errorf("failed to generate worktree name for branch %q: %w", branchName, err)
	}

	if _, err = fmt.Fprintln(w, name); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}
