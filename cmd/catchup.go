package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/spf13/cobra"
)

var catchupCmd = &cobra.Command{
	Use:   "catchup",
	Short: "Merge the latest remote root branch into the current feature branch",
	Long: `Catchup fetches the remote root branch (e.g., origin/main) and merges it
into the current feature branch. This keeps your feature branch up to date
with the latest changes from the main line of development.

The root branch is determined by querying the remote's HEAD reference.

The worktree must be in a clean state (no uncommitted changes). If it is
dirty, commit, stash, or reset your changes before running catchup.`,
	Args: cobra.NoArgs,
	RunE: runCatchup,
}

func init() {
	catchupCmd.GroupID = "git"
	rootCmd.AddCommand(catchupCmd)
}

type catchupContext struct {
	gitClient git.Git
}

func runCatchup(cmd *cobra.Command, _ []string) error {
	originalCwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	rt, err := loadCommandRuntime()
	if err != nil {
		return err
	}

	if rt.cwd != originalCwd {
		return errors.New("catchup must be run from inside a worktree, not from the workspace root")
	}

	ctx := &catchupContext{
		gitClient: rt.gitClient,
	}

	return executeCatchup(cmd.OutOrStdout(), ctx)
}

func executeCatchup(w io.Writer, ctx *catchupContext) error {
	currentBranch, err := ctx.gitClient.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	if currentBranch == "HEAD" {
		return errors.New("cannot catch up: detached HEAD state")
	}

	remoteName, err := ctx.gitClient.GetDefaultRemote("origin")
	if err != nil {
		return fmt.Errorf("failed to get default remote: %w", err)
	}

	rootBranch, err := ctx.gitClient.GetRepoDefaultBranch(remoteName)
	if err != nil {
		return fmt.Errorf("failed to determine root branch: %w", err)
	}
	if rootBranch == "" {
		return fmt.Errorf("could not determine root branch; run 'git remote set-head %s --auto' to configure", remoteName)
	}

	if currentBranch == rootBranch {
		return fmt.Errorf("already on root branch %q; use 'grove sync' instead", rootBranch)
	}

	if _, err := ctx.gitClient.FetchRemote(remoteName); err != nil {
		return fmt.Errorf("failed to fetch remote %q: %w", remoteName, err)
	}

	worktreeRoot, err := ctx.gitClient.GetWorktreeRoot()
	if err != nil {
		return fmt.Errorf("failed to get worktree root: %w", err)
	}

	dirty, err := ctx.gitClient.IsWorktreeDirty(worktreeRoot)
	if err != nil {
		return fmt.Errorf("failed to check worktree state: %w", err)
	}
	if dirty {
		return errors.New("worktree has uncommitted changes; commit, stash, or reset before running catchup")
	}

	mergeRef := remoteName + "/" + rootBranch
	output, err := ctx.gitClient.Merge(mergeRef)
	if err != nil {
		return fmt.Errorf("failed to merge %s: %w", mergeRef, err)
	}

	if output != "" {
		if _, err := fmt.Fprintln(w, output); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "Merged %s into %s\n", mergeRef, currentBranch)
	return err
}
