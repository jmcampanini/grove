package cmd

import (
	"fmt"
	"io"

	"github.com/jmcampanini/grove/internal/naming"
	"github.com/spf13/cobra"
)

func newNamerBranchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "branch <phrase>",
		Short: "Generate a branch name from a phrase",
		Long: `Print the branch name grove create would use for the phrase. The phrase is
slugified with the naming settings, rendered through
local_branch.branch_template as {{.PhraseSlug}}, and capped at
naming.max_length. A phrase that slugifies to nothing is an error.`,
		Args: cobra.ExactArgs(1),
		RunE: runNamerBranch,
	}
}

func runNamerBranch(cmd *cobra.Command, args []string) error {
	cfg, err := loadNamingConfig(cmd)
	if err != nil {
		return err
	}

	namer, err := naming.NewLocalBranchNamer(cfg.LocalBranch, cfg.Naming)
	if err != nil {
		return fmt.Errorf("failed to initialize local branch namer: %w", err)
	}

	ctx := &namerContext{namer: namer}
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
