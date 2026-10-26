package cli

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"charm.land/log/v2"
	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/worktree"
	"github.com/spf13/cobra"
)

func newRemoveCmd(services commandServices) *cobra.Command {
	var force, keepBranch bool
	cmd := &cobra.Command{
		Use:   "remove <target>",
		Short: "Remove a worktree and its branch",
		Long: `Delete a worktree directory and, unless --keep-branch is passed, its
local branch, then print what was removed on stdout.

The target can be:
  - An absolute path to a worktree
  - A worktree directory name (e.g., "wt-my-feature"), which is the
    worktree at the path the active space gives that name (see grove help
    layout); a worktree elsewhere needs its path or branch
  - A branch name (e.g., "feature/my-feature")

A worktree with uncommitted changes is refused unless --force is passed,
which discards those changes and force-deletes the branch even when it is
not merged. Stale worktree entries are pruned from git afterwards. The
main worktree cannot be removed. Nothing prompts.

` + helpFailure,
		Example: `  grove remove wt-my-feature
  grove remove feature/my-feature --keep-branch
  grove remove /path/to/worktree --force`,
		Args:    cobra.ExactArgs(1),
		GroupID: "worktree",
		RunE: func(cmd *cobra.Command, args []string) error {
			return services.runRemove(cmd, args, force, keepBranch)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Discard uncommitted changes and delete an unmerged branch")
	cmd.Flags().BoolVar(&keepBranch, "keep-branch", false, "Keep the local branch after removing the worktree")
	return cmd
}

type removeContext struct {
	cfg              config.Config
	gitClient        git.Git
	logger           *log.Logger
	mainWorktreePath string
}

func (services commandServices) runRemove(cmd *cobra.Command, args []string, force, keepBranch bool) error {
	rt, err := services.loadRuntime(cmd)
	if err != nil {
		return err
	}

	// Root the git client at mainWorktreePath so that post-removal commands
	// (DeleteBranch, PruneWorktrees) still work even when cwd is the removed worktree.
	ctx := &removeContext{
		cfg:              rt.Config,
		gitClient:        git.New(cmd.Context(), false, rt.MainWorktreePath, rt.Config.Git.Timeout, rt.Logger),
		logger:           rt.Logger,
		mainWorktreePath: rt.MainWorktreePath,
	}

	return executeRemove(cmd.OutOrStdout(), ctx, args[0], force, keepBranch)
}

func executeRemove(w io.Writer, ctx *removeContext, target string, force, keepBranch bool) error {
	worktrees, err := ctx.gitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	wt, err := resolveTarget(target, worktrees, spaceWorktreePath(ctx, target))
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

	branchName := worktree.ExtractBranchName(wt)

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

// spaceWorktreePath renders where the active space would place a worktree
// named target, or "" when that cannot be determined. Removal must keep
// working for worktrees outside the layout, so a layout that cannot render
// (unset root variable, no remote) only disables the space lookup.
func spaceWorktreePath(ctx *removeContext, target string) string {
	path, err := worktree.ResolvePath(ctx.cfg, ctx.gitClient, target)
	if err != nil {
		ctx.logger.Debug("cannot render the active space path; a bare name will not match", "target", target, "err", err)
		return ""
	}
	return path
}

// resolveTarget finds the worktree for target: an absolute path, the worktree
// at spacePath (where the active space places a worktree named target), or a
// branch name. A name is never matched outside the active space.
func resolveTarget(target string, worktrees []git.Worktree, spacePath string) (*git.Worktree, error) {
	if wt := worktreeAtPath(worktrees, target); wt != nil {
		return wt, nil
	}
	if wt := worktreeAtPath(worktrees, spacePath); wt != nil {
		return wt, nil
	}

	for i := range worktrees {
		if name := worktree.ExtractBranchName(&worktrees[i]); name == target {
			return &worktrees[i], nil
		}
	}

	return nil, fmt.Errorf("no worktree found matching %q", target)
}

// worktreeAtPath returns the worktree whose path equals path. git worktree
// list prints resolved paths, so both sides are compared after resolving
// symlinks; on macOS the temp and /var trees are symlinks into /private. An
// empty path matches nothing.
func worktreeAtPath(worktrees []git.Worktree, path string) *git.Worktree {
	if path == "" {
		return nil
	}
	resolved := canonicalPath(path)
	for i := range worktrees {
		if canonicalPath(worktrees[i].AbsolutePath) == resolved {
			return &worktrees[i]
		}
	}
	return nil
}

// canonicalPath resolves symlinks in path, or returns it unchanged when it
// does not exist.
func canonicalPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}
