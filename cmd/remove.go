package cmd

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/git"
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
  - A worktree directory name (e.g., "wt-my-feature"), looked up first at
    the path the active space would give it (see grove help layout), then
    anywhere in the repository when exactly one worktree has that name
  - A branch name (e.g., "feature/my-feature")

A name that only exists outside the active space and is shared by several
worktrees is ambiguous; pass the path instead.

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
	cfg              config.Config
	gitClient        git.Git
	logger           *log.Logger
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
		cfg:              rt.cfg,
		gitClient:        git.New(cmd.Context(), false, rt.mainWorktreePath, rt.cfg.Git.Timeout, rt.logger),
		logger:           rt.logger,
		mainWorktreePath: rt.mainWorktreePath,
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

// spaceWorktreePath renders where the active space would place a worktree
// named target, or "" when that cannot be determined. Removal must keep
// working for worktrees outside the layout, so a layout that cannot render
// (unset root variable, no remote) only disables the space lookup.
func spaceWorktreePath(ctx *removeContext, target string) string {
	path, err := resolveWorktreePath(ctx.cfg, ctx.gitClient, target)
	if err != nil {
		ctx.logger.Debug("cannot render the active space path; matching names across all worktrees", "target", target, "err", err)
		return ""
	}
	return path
}

// resolveTarget finds the worktree for target: an absolute path, the name of
// the worktree at spacePath (the active space's path for that name), a name
// held by exactly one worktree anywhere, or a branch name. A name shared by
// several worktrees outside the active space is ambiguous.
func resolveTarget(target string, worktrees []git.Worktree, spacePath string) (*git.Worktree, error) {
	// git worktree list prints resolved paths, so compare resolved forms;
	// on macOS the temp and /var trees are symlinks into /private.
	if wt := worktreeAtPath(worktrees, target); wt != nil {
		return wt, nil
	}
	if wt := worktreeAtPath(worktrees, spacePath); wt != nil {
		return wt, nil
	}

	var byName []*git.Worktree
	for i := range worktrees {
		if filepath.Base(worktrees[i].AbsolutePath) == target {
			byName = append(byName, &worktrees[i])
		}
	}
	if len(byName) == 1 {
		return byName[0], nil
	}

	for i := range worktrees {
		if name := extractBranchName(&worktrees[i]); name == target {
			return &worktrees[i], nil
		}
	}

	if len(byName) > 1 {
		paths := make([]string, 0, len(byName))
		for _, wt := range byName {
			paths = append(paths, wt.AbsolutePath)
		}
		return nil, fmt.Errorf("worktree name %q is ambiguous; pass one of these paths instead:\n  %s", target, strings.Join(paths, "\n  "))
	}

	return nil, fmt.Errorf("no worktree found matching %q", target)
}

// worktreeAtPath returns the worktree whose path equals path, comparing the
// symlink-resolved forms when the path exists. An empty path matches nothing.
func worktreeAtPath(worktrees []git.Worktree, path string) *git.Worktree {
	if path == "" {
		return nil
	}
	resolved := canonicalPath(path)
	for i := range worktrees {
		if worktrees[i].AbsolutePath == path || canonicalPath(worktrees[i].AbsolutePath) == resolved {
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
	return deleteBranchIfExists(gitClient, branchName)
}

func deleteBranchIfExists(gitClient git.Git, branchName string) error {
	if branchName == "" {
		return nil
	}
	if err := gitClient.DeleteBranch(branchName, true); err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to delete branch %q: %w", branchName, err)
	}
	return nil
}
