package cmd

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	var force, keepBranch bool
	cmd := &cobra.Command{
		Use:   "remove <target>",
		Short: "Remove a worktree and its branch",
		Long: `Remove removes a worktree and optionally deletes its local branch.

The target can be:
  - An absolute path to a worktree
  - A directory name within the workspace (e.g., "wt-my-feature")
  - A branch name (e.g., "feature/my-feature")

By default, removes both the worktree and its local branch.
Use --keep-branch to preserve the branch after removing the worktree.

The main worktree cannot be removed.`,
		Args:    cobra.ExactArgs(1),
		GroupID: "worktree",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(cmd, args, force, keepBranch)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force removal even with uncommitted changes")
	cmd.Flags().BoolVar(&keepBranch, "keep-branch", false, "Keep the local branch after removing the worktree")
	return cmd
}

type removeContext struct {
	gitClient        git.Git
	mainWorktreePath string
}

func runRemove(cmd *cobra.Command, args []string, force, keepBranch bool) error {
	rt, err := loadCommandRuntime(cmd)
	if err != nil {
		return err
	}

	// Root the git client at mainWorktreePath so that post-removal commands
	// (DeleteBranch, PruneWorktrees) still work even when cwd is the removed worktree.
	ctx := &removeContext{
		gitClient:        git.New(cmd.Context(), false, rt.mainWorktreePath, rt.cfg.Git.Timeout, rt.logger),
		mainWorktreePath: rt.mainWorktreePath,
	}

	return executeRemove(cmd.OutOrStdout(), ctx, args[0], force, keepBranch)
}

func executeRemove(w io.Writer, ctx *removeContext, target string, force, keepBranch bool) error {
	worktrees, err := ctx.gitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	workspacePath, err := ctx.gitClient.GetWorkspacePath()
	if err != nil {
		return fmt.Errorf("failed to get workspace path: %w", err)
	}

	wt, err := resolveTarget(target, worktrees, workspacePath)
	if err != nil {
		return err
	}

	if wt.AbsolutePath == ctx.mainWorktreePath {
		return errors.New("cannot remove the main worktree")
	}

	if !force {
		dirty, err := ctx.gitClient.IsWorktreeDirty(wt.AbsolutePath)
		if err != nil {
			return fmt.Errorf("failed to check worktree state: %w", err)
		}
		if dirty {
			return fmt.Errorf("worktree %q has uncommitted changes; use --force to remove anyway", filepath.Base(wt.AbsolutePath))
		}
	}

	branchName := extractBranchName(wt)

	if err := ctx.gitClient.RemoveWorktree(wt.AbsolutePath, force); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	if !keepBranch && branchName != "" {
		if err := ctx.gitClient.DeleteBranch(branchName, force); err != nil {
			return fmt.Errorf("failed to delete branch %q: %w", branchName, err)
		}
	}

	if err := ctx.gitClient.PruneWorktrees(); err != nil {
		return fmt.Errorf("failed to prune worktrees: %w", err)
	}

	msg := "Removed worktree " + filepath.Base(wt.AbsolutePath)
	if !keepBranch && branchName != "" {
		msg += " and branch " + branchName
	}
	_, err = fmt.Fprintln(w, msg)
	return err
}

func resolveTarget(target string, worktrees []git.Worktree, workspacePath string) (*git.Worktree, error) {
	for i := range worktrees {
		if worktrees[i].AbsolutePath == target {
			return &worktrees[i], nil
		}
	}

	absTarget := filepath.Join(workspacePath, target)
	for i := range worktrees {
		if worktrees[i].AbsolutePath == absTarget {
			return &worktrees[i], nil
		}
	}

	for i := range worktrees {
		if name := extractBranchName(&worktrees[i]); name == target {
			return &worktrees[i], nil
		}
	}

	return nil, fmt.Errorf("no worktree found matching %q", target)
}

func extractBranchName(wt *git.Worktree) string {
	if wt.Ref == nil {
		return ""
	}
	if branch, ok := wt.Ref.FullBranch(); ok {
		return branch.Name
	}
	return ""
}

func removeWorktreeAndBranch(gitClient git.Git, absPath, branchName string) error {
	if err := gitClient.RemoveWorktree(absPath, true); err != nil {
		return fmt.Errorf("failed to remove worktree %q: %w", filepath.Base(absPath), err)
	}
	if branchName != "" {
		if err := gitClient.DeleteBranch(branchName, true); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return fmt.Errorf("failed to delete branch %q: %w", branchName, err)
			}
		}
	}
	return nil
}
